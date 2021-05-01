package main

import "html/template"

const layout = "templates/layout.tmpl"

var (
	loginTemplate   = template.Must(template.ParseFiles("templates/login.tmpl"))
	indexTemplate   = template.Must(template.ParseFiles(layout, "templates/index.tmpl"))
	mypageTemplate  = template.Must(template.ParseFiles(layout, "templates/mypage.tmpl"))
	productTemplate = template.Must(template.ParseFiles(layout, "templates/product.tmpl"))
)
