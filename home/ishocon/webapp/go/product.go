package main

import (
	"context"
	"sync"
)

var (
	productCache map[int]*Product
	pageCache    sync.Map
)

// Product Model
type Product struct {
	ID               int
	Name             string
	Description      string
	ShortDescription string
	ImagePath        string
	Price            int
	CreatedAt        string
}

// ProductWithComments Model
type ProductWithComments struct {
	ID               int
	Name             string
	Description      string
	ShortDescription string
	ImagePath        string
	Price            int
	CreatedAt        string
	CommentCount     int
	Comments         []CommentWriter
}

// CommentWriter Model
type CommentWriter struct {
	Content      string
	ShortContent string
	Writer       string
}

func getProduct(pctx context.Context, pid int) Product {
	ctx, span := tracer.Start(pctx, "getProduct")
	defer span.End()

	if p, ok := productCache[pid]; ok {
		return *p
	}

	p := Product{}
	row := db.QueryRowContext(ctx, "SELECT * FROM products WHERE id = ? LIMIT 1", pid)
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.ImagePath, &p.Price, &p.CreatedAt)
	if err != nil {
		panic(err.Error())
	}

	return p
}

func getProductsWithCommentsAt(pctx context.Context, page int) []ProductWithComments {
	ctx, span := tracer.Start(pctx, "getProductsWithCommentsAt")
	defer span.End()

	// select 50 products with offset page*50
	var ids []int
	if v, ok := pageCache.Load(page); ok {
		ids = v.([]int)
	} else {
		rows, err := db.QueryContext(ctx, "SELECT id FROM products ORDER BY id DESC LIMIT 50 OFFSET ?", page*50)
		if err != nil {
			return nil
		}

		defer rows.Close()
		for rows.Next() {
			var id int
			rows.Scan(&id)
			ids = append(ids, id)
		}
		pageCache.Store(page, ids)
	}

	products := []ProductWithComments{}
	for _, id := range ids {
		p := ProductWithComments{ID: id}
		product := getProduct(ctx, id)
		p.Name, p.Description, p.ShortDescription, p.ImagePath, p.Price, p.CreatedAt = product.Name, product.Description, product.ShortDescription, product.ImagePath, product.Price, product.CreatedAt

		var comments []Comment
		if v, ok := commentCache.Load(p.ID); ok {
			comments = v.([]Comment)
			p.CommentCount = len(comments)
		}

		if p.CommentCount > 0 {
			// select 5 comments and its writer for the product
			var cWriters []CommentWriter

			if len(comments) > 5 {
				comments = comments[:5]
			}
			for _, c := range comments {
				u, _ := userCache.Load(c.UserID)
				cWriters = append(cWriters, CommentWriter{
					Content: c.Content,
					Writer:  u.(User).Name,
				})
			}

			p.Comments = cWriters
		}

		products = append(products, p)
	}

	return products
}

func (p *Product) isBought(uid int) bool {
	if v, ok := historyCache.Load(uid); ok {
		for _, h := range v.([]userHistory) {
			if h.ProductID == p.ID {
				return true
			}
		}
	}
	return false
}
