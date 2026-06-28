package bootstrap

import (
	"go-mall/internal/model"

	"gorm.io/gorm"
)

func SeedDefaultData(db *gorm.DB) error {
	// 1. 如果merchants 表里已经有 id为1的商户数据，则不再插入默认数据
	var count int64
	if err := db.Model(&model.Merchant{}).Where("id = ?", 1).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// 2. 创建默认商户
	merchant := model.Merchant{
		ID:   1,
		Name: "默认商户",
		Logo: "https://example.com/default-logo.png",
	}
	if err := db.Create(&merchant).Error; err != nil {
		return err
	}

	// 3. 创建几个分类
	categories := []model.Category{
		{MerchantID: merchant.ID, ParentID: 0, Name: "数码", Sort: 1, Status: 1},
		{MerchantID: merchant.ID, ParentID: 0, Name: "服饰", Sort: 2, Status: 1},
		{MerchantID: merchant.ID, ParentID: 0, Name: "食品", Sort: 3, Status: 1},
	}
	if err := db.Create(&categories).Error; err != nil {
		return err
	}

	// 4. 创建几个商品
	products := []model.Product{
		{MerchantID: merchant.ID, CategoryID: categories[0].ID, Name: "机械键盘", Description: "高品质机械键盘", Status: 1},
		{MerchantID: merchant.ID, CategoryID: categories[1].ID, Name: "基础白 T", Description: "舒适百搭基础白 T", Status: 1},
		{MerchantID: merchant.ID, CategoryID: categories[2].ID, Name: "手冲咖啡豆", Description: "适合手冲的精选咖啡豆", Status: 1},
	}
	if err := db.Create(&products).Error; err != nil {
		return err
	}

	// 5. 创建几个商品SKU
	productSKUs := []model.ProductSKU{
		{MerchantID: merchant.ID, ProductID: products[0].ID, Name: "机械键盘 - 茶轴", Price: 69900, Stock: 50, Status: 1},
		{MerchantID: merchant.ID, ProductID: products[0].ID, Name: "机械键盘 - 红轴", Price: 69900, Stock: 50, Status: 1},
		{MerchantID: merchant.ID, ProductID: products[1].ID, Name: "基础白 T - M", Price: 1999, Stock: 100, Status: 1},
		{MerchantID: merchant.ID, ProductID: products[1].ID, Name: "基础白 T - L", Price: 1999, Stock: 100, Status: 1},
		{MerchantID: merchant.ID, ProductID: products[2].ID, Name: "手冲咖啡豆 - 250g", Price: 49900, Stock: 50, Status: 1},
	}
	if err := db.Create(&productSKUs).Error; err != nil {
		return err
	}

	return nil
}
