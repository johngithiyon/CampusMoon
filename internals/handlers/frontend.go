package handlers

import (
	"html/template"
	"net/http"
)

func ServeHome(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.New("index.html").ParseFiles("templates/index.html")
	tmpl.Execute(w, nil)
}

func ServeMeet(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.New("meet.html").ParseFiles("templates/meet.html")
	tmpl.Execute(w, nil)
}

func ServeAdmin(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.New("admin.html").ParseFiles("templates/admin.html")
	tmpl.Execute(w, nil)
}

func ServeStaff(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.New("staff.html").ParseFiles("templates/staff.html")
	tmpl.Execute(w, nil)
}

func ServeStudent(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.New("student.html").ParseFiles("templates/student.html")
	tmpl.Execute(w, nil)
}

func ServeWelcome(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.New("welcome.html").ParseFiles("templates/welcome.html")
	tmpl.Execute(w, nil)
}

func Cs(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/cs.html")
}

func Serveelabs(w http.ResponseWriter,r *http.Request) {
	http.ServeFile(w,r,"templates/e.html")
}

func ServeConnect(w http.ResponseWriter, r *http.Request) {
     http.ServeFile(w, r, "templates/connect_meet.html")
}