package main

import (
	"sync"
)

var productCache sync.Map

// Product Model
type Product struct {
	ID          int
	Name        string
	Description string
	ImagePath   string
	Price       int
	CreatedAt   string
}

// ProductWithComments Model
type ProductWithComments struct {
	ID           int
	Name         string
	Description  string
	ImagePath    string
	Price        int
	CreatedAt    string
	CommentCount int
	Comments     []CommentWriter
}

// CommentWriter Model
type CommentWriter struct {
	Content string
	Writer  string
}

func getProduct(pid int) Product {
	if v, ok := productCache.Load(pid); ok {
		return v.(Product)
	}

	p := Product{}
	row := db.QueryRow("SELECT * FROM products WHERE id = ? LIMIT 1", pid)
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.ImagePath, &p.Price, &p.CreatedAt)
	if err != nil {
		panic(err.Error())
	}

	return p
}

func getProductsWithCommentsAt(page int) []ProductWithComments {
	// select 50 products with offset page*50
	products := []ProductWithComments{}
	rows, err := db.Query("SELECT id FROM products ORDER BY id DESC LIMIT 50 OFFSET ?", page*50)
	if err != nil {
		return nil
	}

	defer rows.Close()
	for rows.Next() {
		p := ProductWithComments{}
		err = rows.Scan(&p.ID)

		product := getProduct(p.ID)
		p.Name, p.Description, p.ImagePath, p.Price, p.CreatedAt = product.Name, product.Description, product.ImagePath, product.Price, product.CreatedAt

		if v, ok := commentCache.Load(p.ID); ok {
			p.CommentCount = len(v.([]Comment))
		}

		if p.CommentCount > 0 {
			// select 5 comments and its writer for the product
			var cWriters []CommentWriter

			subrows, suberr := db.Query("SELECT * FROM comments as c INNER JOIN users as u "+
				"ON c.user_id = u.id WHERE c.product_id = ? ORDER BY c.created_at DESC LIMIT 5", p.ID)
			if suberr != nil {
				subrows = nil
			}

			defer subrows.Close()
			for subrows.Next() {
				var i int
				var s string
				var cw CommentWriter
				subrows.Scan(&i, &i, &i, &cw.Content, &s, &i, &cw.Writer, &s, &s, &s)
				cWriters = append(cWriters, cw)
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
