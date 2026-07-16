package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	ErrTradeNotFound        = errors.New("交易不存在")
	ErrTradeConflict        = errors.New("交易状态冲突")
	ErrTradePreviewRequired = errors.New("请重新预览交易")
)

const acquireTradePreviewScript = `
local value = redis.call("GET", KEYS[1])
if not value then
	return "missing"
end
if string.sub(value, 1, 8) == "pending|" then
	redis.call("SET", KEYS[1], "processing", "EX", ARGV[1])
	return value
end
return value
`

type TradeService interface {
	Preview(ctx context.Context, userID int64, req dto.TradePreviewRequest) (*dto.TradePreviewResponse, error)
	Create(ctx context.Context, userID int64, req dto.CreateTradeRequest) (*dto.TradeResponse, error)
	List(ctx context.Context, userID int64, req dto.TradeListRequest) (*dto.PageResponse[dto.TradeResponse], error)
	Detail(ctx context.Context, userID, tradeID int64) (*dto.TradeResponse, error)
	Cancel(ctx context.Context, userID, tradeID int64) (*dto.TradeResponse, error)
	ResolveOrderTrade(ctx context.Context, userID, orderID int64) (tradeID int64, childCount int64, linked bool, err error)
}

type tradePaymentCancellationPreparer interface {
	PrepareUserTradeForCancel(ctx context.Context, userID int64, tradeID int64) error
}

type tradeService struct {
	repo                repository.TradeRepository
	redisClient         *redis.Client
	paymentCancellation tradePaymentCancellationPreparer
	now                 func() time.Time
}

func NewTradeService(repo repository.TradeRepository, redisClient *redis.Client, paymentCancellation tradePaymentCancellationPreparer) TradeService {
	return &tradeService{repo: repo, redisClient: redisClient, paymentCancellation: paymentCancellation, now: time.Now}
}

func tradePreviewKey(userID int64, token string) string {
	return fmt.Sprintf("mall:trade:idempotency:%d:%s", userID, token)
}

func generateTradeNo() string {
	return fmt.Sprintf("T%s%s", time.Now().Format("20060102150405"), uuid.NewString()[:8])
}

func calculateCommissionAmount(payableAmount int64, rateBPS int) (int64, error) {
	if payableAmount < 0 || rateBPS < 0 || rateBPS > 10000 {
		return 0, fmt.Errorf("佣金配置不合法")
	}
	// Split the quotient and remainder so payableAmount*rateBPS cannot overflow int64.
	return payableAmount/10000*int64(rateBPS) + payableAmount%10000*int64(rateBPS)/10000, nil
}

func tradeStatusText(status int) string {
	switch status {
	case model.TradeStatusPendingPayment:
		return "待支付"
	case model.TradeStatusPaid:
		return "已支付"
	case model.TradeStatusClosed:
		return "已关闭"
	case model.TradeStatusPartiallyRefunded:
		return "部分退款"
	case model.TradeStatusRefunded:
		return "已退款"
	default:
		return "未知状态"
	}
}

func (s *tradeService) Preview(ctx context.Context, userID int64, req dto.TradePreviewRequest) (*dto.TradePreviewResponse, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}
	input, err := normalizeTradeInput(req.Items, req.MerchantCoupons)
	if err != nil {
		return nil, err
	}
	quote, err := buildTradeQuote(ctx, s.repo, userID, req.AddressID, input, false, s.now())
	if err != nil {
		return nil, err
	}
	fingerprint, err := tradeQuoteFingerprint(quote)
	if err != nil {
		return nil, err
	}
	token := uuid.NewString()
	if err := s.redisClient.Set(ctx, tradePreviewKey(userID, token), "pending|"+fingerprint, 15*time.Minute).Err(); err != nil {
		return nil, fmt.Errorf("保存交易预览失败，请稍后重试")
	}
	return quotePreviewResponse(quote, token), nil
}

func (s *tradeService) Create(ctx context.Context, userID int64, req dto.CreateTradeRequest) (*dto.TradeResponse, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}
	if req.AddressID <= 0 {
		return nil, fmt.Errorf("收货地址不能为空")
	}
	if _, err := uuid.Parse(req.IdempotencyToken); err != nil {
		return nil, fmt.Errorf("幂等 token 不合法")
	}
	if utf8.RuneCountInString(req.Remark) > 255 {
		return nil, fmt.Errorf("订单备注不能超过 255 个字符")
	}
	input, err := normalizeTradeInput(req.Items, req.MerchantCoupons)
	if err != nil {
		return nil, err
	}

	if existing, err := s.findExistingTrade(ctx, userID, req.IdempotencyToken); err != nil {
		return nil, err
	} else if existing != nil {
		return s.loadTradeResponse(ctx, existing)
	}

	tokenKey := tradePreviewKey(userID, req.IdempotencyToken)
	state, err := s.acquirePreview(ctx, tokenKey)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(state, "trade:") {
		tradeID, parseErr := strconv.ParseInt(strings.TrimPrefix(state, "trade:"), 10, 64)
		if parseErr == nil && tradeID > 0 {
			return s.Detail(ctx, userID, tradeID)
		}
	}
	if state == "missing" {
		if existing, findErr := s.findExistingTrade(ctx, userID, req.IdempotencyToken); findErr != nil {
			return nil, findErr
		} else if existing != nil {
			return s.loadTradeResponse(ctx, existing)
		}
		return nil, fmt.Errorf("%w: 交易预览已过期", ErrTradePreviewRequired)
	}
	if !strings.HasPrefix(state, "pending|") {
		return nil, fmt.Errorf("%w: 交易正在处理，请勿重复提交", ErrTradeConflict)
	}
	expectedFingerprint := strings.TrimPrefix(state, "pending|")

	var createdTrade *model.Trade
	err = s.repo.Transaction(ctx, func(repo repository.TradeRepository) error {
		quote, err := buildTradeQuote(ctx, repo, userID, req.AddressID, input, true, s.now())
		if err != nil {
			return err
		}
		fingerprint, err := tradeQuoteFingerprint(quote)
		if err != nil {
			return err
		}
		if fingerprint != expectedFingerprint {
			return fmt.Errorf("%w: 地址、商品价格、库存或优惠券发生变化", ErrTradePreviewRequired)
		}

		trade := &model.Trade{
			TradeNo:        generateTradeNo(),
			UserID:         userID,
			Status:         model.TradeStatusPendingPayment,
			GoodsAmount:    quote.GoodsAmount,
			FreightAmount:  quote.FreightAmount,
			DiscountAmount: quote.DiscountAmount,
			PayableAmount:  quote.PayableAmount,
			IdempotencyKey: req.IdempotencyToken,
		}
		if err := repo.CreateTrade(ctx, trade); err != nil {
			return err
		}

		inventoryLogs := make([]model.InventoryLog, 0, len(input.SKUIds))
		for _, group := range quote.Groups {
			tradeID := trade.ID
			merchantName := group.Merchant.Name
			commissionRate := group.Merchant.CommissionRateBPS
			commissionAmount, err := calculateCommissionAmount(group.PayableAmount, commissionRate)
			if err != nil {
				return fmt.Errorf("商户 %d %w", group.Merchant.ID, err)
			}
			settlementAmount := group.PayableAmount - commissionAmount
			order := &model.Order{
				TradeID:           &tradeID,
				OrderNo:           generateOrderNo(),
				UserID:            userID,
				MerchantID:        group.Merchant.ID,
				MerchantName:      &merchantName,
				Status:            model.OrderStatusPendingPayment,
				ReceiverName:      quote.Address.ReceiverName,
				ReceiverPhone:     quote.Address.ReceiverPhone,
				ReceiverAddress:   buildReceiverAddress(&quote.Address),
				GoodsAmount:       group.GoodsAmount,
				FreightAmount:     group.FreightAmount,
				DiscountAmount:    group.DiscountAmount,
				PayableAmount:     group.PayableAmount,
				CommissionRateBPS: &commissionRate,
				CommissionAmount:  &commissionAmount,
				SettlementAmount:  &settlementAmount,
				UserCouponID:      group.UserCouponID,
				Remark:            strings.TrimSpace(req.Remark),
			}
			if err := repo.CreateOrder(ctx, order); err != nil {
				return err
			}

			items := make([]model.OrderItem, 0, len(group.Items))
			for _, quoteItem := range group.Items {
				afterStock, err := repo.DecreaseSKUStock(ctx, group.Merchant.ID, quoteItem.SKU.ID, quoteItem.Response.Quantity)
				if err != nil {
					return err
				}
				items = append(items, model.OrderItem{
					OrderID:        order.ID,
					ProductID:      quoteItem.Product.ID,
					SKUID:          quoteItem.SKU.ID,
					ProductName:    quoteItem.Response.ProductName,
					SKUName:        quoteItem.Response.SKUName,
					SKUImage:       quoteItem.Response.SKUImage,
					Price:          quoteItem.Response.Price,
					Quantity:       quoteItem.Response.Quantity,
					Subtotal:       quoteItem.Response.Subtotal,
					DiscountAmount: quoteItem.Response.DiscountAmount,
					PayableAmount:  quoteItem.Response.PayableAmount,
				})
				inventoryLogs = append(inventoryLogs, model.InventoryLog{
					MerchantID:    group.Merchant.ID,
					ProductID:     quoteItem.Product.ID,
					SKUID:         quoteItem.SKU.ID,
					ProductName:   quoteItem.Response.ProductName,
					SKUName:       quoteItem.Response.SKUName,
					ChangeType:    model.InventoryChangeOrderCreate,
					Quantity:      -quoteItem.Response.Quantity,
					BeforeStock:   afterStock + quoteItem.Response.Quantity,
					AfterStock:    afterStock,
					ReferenceType: model.InventoryReferenceOrder,
					ReferenceID:   order.ID,
					OperatorType:  model.InventoryOperatorUser,
					OperatorID:    userID,
					Remark:        "创建多商户交易扣减库存",
				})
			}
			if err := repo.CreateOrderItems(ctx, items); err != nil {
				return err
			}
			if group.UserCouponID > 0 {
				if err := repo.UseUserCoupon(ctx, group.UserCouponID, userID, order.ID, s.now()); err != nil {
					return err
				}
				if err := repo.IncrementCouponUsed(ctx, group.CouponID, 1); err != nil {
					return err
				}
			}
		}
		if err := repo.CreateInventoryLogs(ctx, inventoryLogs); err != nil {
			return err
		}
		createdTrade = trade
		return nil
	})
	if err != nil {
		if existing, findErr := s.findExistingTrade(ctx, userID, req.IdempotencyToken); findErr == nil && existing != nil {
			_ = s.redisClient.Set(ctx, tokenKey, fmt.Sprintf("trade:%d", existing.ID), 24*time.Hour).Err()
			return s.loadTradeResponse(ctx, existing)
		}
		_ = s.redisClient.Del(ctx, tokenKey).Err()
		return nil, err
	}

	_ = s.redisClient.Set(ctx, tokenKey, fmt.Sprintf("trade:%d", createdTrade.ID), 24*time.Hour).Err()
	fields := make([]string, 0, len(input.SKUIds))
	for _, skuID := range input.SKUIds {
		fields = append(fields, strconv.FormatInt(skuID, 10))
	}
	if len(fields) > 0 {
		_ = s.redisClient.HDel(ctx, cartKey(userID), fields...).Err()
	}
	return s.loadTradeResponse(ctx, createdTrade)
}

func (s *tradeService) acquirePreview(ctx context.Context, key string) (string, error) {
	value, err := s.redisClient.Eval(ctx, acquireTradePreviewScript, []string{key}, int((15 * time.Minute).Seconds())).Result()
	if err != nil {
		return "", fmt.Errorf("交易幂等校验失败，请稍后重试")
	}
	state, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("交易幂等校验失败，请稍后重试")
	}
	return state, nil
}

func (s *tradeService) findExistingTrade(ctx context.Context, userID int64, key string) (*model.Trade, error) {
	trade, err := s.repo.FindTradeByIdempotencyKey(ctx, userID, key)
	if err == nil {
		return trade, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

func (s *tradeService) List(ctx context.Context, userID int64, req dto.TradeListRequest) (*dto.PageResponse[dto.TradeResponse], error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}
	if req.Status < 0 || req.Status > model.TradeStatusRefunded {
		return nil, fmt.Errorf("交易状态不合法")
	}
	page, pageSize := normalizeOrderPage(req.Page, req.PageSize)
	trades, total, err := s.repo.ListTradesByUserID(ctx, userID, req.Status, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	tradeIDs := make([]int64, 0, len(trades))
	for _, trade := range trades {
		tradeIDs = append(tradeIDs, trade.ID)
	}
	orders, err := s.repo.FindOrdersByTradeIDs(ctx, tradeIDs, userID)
	if err != nil {
		return nil, err
	}
	orderIDs := make([]int64, 0, len(orders))
	ordersByTrade := make(map[int64][]model.Order, len(trades))
	for _, order := range orders {
		if order.TradeID == nil {
			return nil, fmt.Errorf("订单 %d 缺少交易引用", order.ID)
		}
		orderIDs = append(orderIDs, order.ID)
		ordersByTrade[*order.TradeID] = append(ordersByTrade[*order.TradeID], order)
	}
	items, err := s.repo.FindOrderItemsByOrderIDs(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	itemsByOrder := make(map[int64][]model.OrderItem, len(orders))
	for _, item := range items {
		itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], item)
	}
	list := make([]dto.TradeResponse, 0, len(trades))
	for _, trade := range trades {
		tradeOrders := ordersByTrade[trade.ID]
		if len(tradeOrders) == 0 {
			return nil, fmt.Errorf("交易 %d 没有子订单", trade.ID)
		}
		orderResponses := make([]dto.OrderResponse, 0, len(tradeOrders))
		for _, order := range tradeOrders {
			orderResponses = append(orderResponses, *toOrderResponse(order, itemsByOrder[order.ID]))
		}
		list = append(list, *toTradeResponse(trade, orderResponses))
	}
	return &dto.PageResponse[dto.TradeResponse]{List: list, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *tradeService) Detail(ctx context.Context, userID, tradeID int64) (*dto.TradeResponse, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}
	if tradeID <= 0 {
		return nil, fmt.Errorf("交易 ID 不合法")
	}
	trade, err := s.repo.FindTradeByIDAndUserID(ctx, tradeID, userID, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTradeNotFound
		}
		return nil, err
	}
	return s.loadTradeResponse(ctx, trade)
}

func (s *tradeService) Cancel(ctx context.Context, userID, tradeID int64) (*dto.TradeResponse, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}
	if tradeID <= 0 {
		return nil, fmt.Errorf("交易 ID 不合法")
	}
	trade, err := s.repo.FindTradeByIDAndUserID(ctx, tradeID, userID, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTradeNotFound
		}
		return nil, err
	}
	if trade.Status == model.TradeStatusClosed {
		return s.loadTradeResponse(ctx, trade)
	}
	if trade.Status != model.TradeStatusPendingPayment {
		return nil, fmt.Errorf("%w: 已支付或已退款交易不能取消", ErrTradeConflict)
	}
	if s.paymentCancellation != nil {
		if err := s.paymentCancellation.PrepareUserTradeForCancel(ctx, userID, trade.ID); err != nil {
			return nil, err
		}
	}
	err = s.repo.Transaction(ctx, func(repo repository.TradeRepository) error {
		trade, err := repo.FindTradeByIDAndUserID(ctx, tradeID, userID, true)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTradeNotFound
			}
			return err
		}
		if trade.Status == model.TradeStatusClosed {
			return nil
		}
		if trade.Status != model.TradeStatusPendingPayment {
			return fmt.Errorf("%w: 已支付或已退款交易不能取消", ErrTradeConflict)
		}
		orders, err := repo.FindOrdersByTradeID(ctx, trade.ID, userID, true)
		if err != nil {
			return err
		}
		if len(orders) == 0 {
			return fmt.Errorf("%w: 交易没有子订单", ErrTradeConflict)
		}
		orderIDs := make([]int64, 0, len(orders))
		orderByID := make(map[int64]model.Order, len(orders))
		for _, order := range orders {
			if order.Status != model.OrderStatusPendingPayment {
				return fmt.Errorf("%w: 子订单 %d 状态已变化", ErrTradeConflict, order.ID)
			}
			orderIDs = append(orderIDs, order.ID)
			orderByID[order.ID] = order
		}
		items, err := repo.FindOrderItemsByOrderIDs(ctx, orderIDs)
		if err != nil {
			return err
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].SKUID == items[j].SKUID {
				return items[i].OrderID < items[j].OrderID
			}
			return items[i].SKUID < items[j].SKUID
		})
		now := s.now()
		if err := repo.ClosePendingPaymentsByTradeID(ctx, trade.ID, now); err != nil {
			return err
		}
		if err := repo.CloseTrade(ctx, trade.ID, userID, now); err != nil {
			return err
		}
		cancelled, err := repo.CancelOrdersByTradeID(ctx, trade.ID, userID, now)
		if err != nil {
			return err
		}
		if cancelled != int64(len(orders)) {
			return fmt.Errorf("%w: 预期取消 %d 张子订单，实际 %d 张", ErrTradeConflict, len(orders), cancelled)
		}

		logs := make([]model.InventoryLog, 0, len(items))
		for _, item := range items {
			order := orderByID[item.OrderID]
			afterStock, err := repo.IncreaseSKUStock(ctx, order.MerchantID, item.SKUID, item.Quantity)
			if err != nil {
				return err
			}
			logs = append(logs, model.InventoryLog{
				MerchantID:    order.MerchantID,
				ProductID:     item.ProductID,
				SKUID:         item.SKUID,
				ProductName:   item.ProductName,
				SKUName:       item.SKUName,
				ChangeType:    model.InventoryChangeOrderCancel,
				Quantity:      item.Quantity,
				BeforeStock:   afterStock - item.Quantity,
				AfterStock:    afterStock,
				ReferenceType: model.InventoryReferenceOrder,
				ReferenceID:   order.ID,
				OperatorType:  model.InventoryOperatorUser,
				OperatorID:    userID,
				Remark:        "取消多商户交易恢复库存",
			})
		}
		for _, order := range orders {
			if order.UserCouponID <= 0 {
				continue
			}
			if err := repo.ReleaseUserCoupon(ctx, order.UserCouponID, userID, order.ID); err != nil {
				return err
			}
			// Coupon ID is read while the user coupon row is locked by the conditional update path.
			userCoupons, err := repo.FindUserCouponsByIDs(ctx, userID, []int64{order.UserCouponID}, true)
			if err != nil || len(userCoupons) != 1 {
				return fmt.Errorf("恢复用户优惠券 %d 失败", order.UserCouponID)
			}
			if err := repo.IncrementCouponUsed(ctx, userCoupons[0].CouponID, -1); err != nil {
				return err
			}
		}
		return repo.CreateInventoryLogs(ctx, logs)
	})
	if err != nil {
		return nil, err
	}
	return s.Detail(ctx, userID, tradeID)
}

func (s *tradeService) ResolveOrderTrade(ctx context.Context, userID, orderID int64) (int64, int64, bool, error) {
	if userID <= 0 {
		return 0, 0, false, fmt.Errorf("用户未登录")
	}
	if orderID <= 0 {
		return 0, 0, false, fmt.Errorf("订单 ID 不合法")
	}
	tradeID, err := s.repo.FindOrderTradeID(ctx, orderID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, false, ErrTradeNotFound
		}
		return 0, 0, false, err
	}
	if tradeID == nil || *tradeID <= 0 {
		return 0, 0, false, nil
	}
	count, err := s.repo.CountOrdersByTradeID(ctx, *tradeID)
	if err != nil {
		return 0, 0, false, err
	}
	if count <= 0 {
		return 0, 0, false, fmt.Errorf("%w: 订单关联的交易没有子订单", ErrTradeConflict)
	}
	return *tradeID, count, true, nil
}

func (s *tradeService) loadTradeResponse(ctx context.Context, trade *model.Trade) (*dto.TradeResponse, error) {
	orders, err := s.repo.FindOrdersByTradeID(ctx, trade.ID, trade.UserID, false)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, fmt.Errorf("交易 %d 没有子订单", trade.ID)
	}
	orderIDs := make([]int64, 0, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
	}
	items, err := s.repo.FindOrderItemsByOrderIDs(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	itemsByOrder := make(map[int64][]model.OrderItem, len(orders))
	for _, item := range items {
		itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], item)
	}
	orderResponses := make([]dto.OrderResponse, 0, len(orders))
	for _, order := range orders {
		orderResponses = append(orderResponses, *toOrderResponse(order, itemsByOrder[order.ID]))
	}
	return toTradeResponse(*trade, orderResponses), nil
}

func toTradeResponse(trade model.Trade, orders []dto.OrderResponse) *dto.TradeResponse {
	return &dto.TradeResponse{
		ID:             trade.ID,
		TradeNo:        trade.TradeNo,
		UserID:         trade.UserID,
		Status:         trade.Status,
		StatusText:     tradeStatusText(trade.Status),
		GoodsAmount:    trade.GoodsAmount,
		FreightAmount:  trade.FreightAmount,
		DiscountAmount: trade.DiscountAmount,
		PayableAmount:  trade.PayableAmount,
		PaidAt:         stringTime(trade.PaidAt),
		ClosedAt:       stringTime(trade.ClosedAt),
		Orders:         orders,
		CreatedAt:      trade.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      trade.UpdatedAt.Format(time.RFC3339),
	}
}
