package handlers

import (
	"net/http"
	"html/template"
)

func ServeLabs(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/labs.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w,nil)
}