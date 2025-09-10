package handlers

import (
	"CampusMoon/storage"
	"html/template"
	"net/http"
)

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Very basic auth check
		row := storage.DB.QueryRow("SELECT code FROM admins WHERE name=$1", username)
		var storedPassword string
		if err := row.Scan(&storedPassword); err != nil || storedPassword != password {
			http.Error(w, "❌ Invalid credentials", http.StatusUnauthorized)
			return
		}

		http.Redirect(w, r, "/welcome", http.StatusSeeOther)
		return
	}

	tmpl, _ := template.New("login.html").ParseFiles("templates/login.html")
	tmpl.Execute(w, nil)
}
