package main

import "text/template"

var (
	mypageTmpl = template.Must(template.ParseFiles("templates/mypage.tmpl"))
)
