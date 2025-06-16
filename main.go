package main

import (
	"log"
	"main/database"
	"main/handler"
	"net/http"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
		return
	}

	db, err := database.GetDatabase(cfg.Postgres)
	if err != nil {
		log.Fatal(err)
		return
	}

	r := handler.NewRestHandler(db)
	http.HandleFunc("/good", r.GetHandler)
	http.HandleFunc("/good/create", r.PostHandler)
	http.HandleFunc("/good/remove", r.DeleteHandler)
	http.HandleFunc("/good/update", r.UpdateHandler)
	http.HandleFunc("/good/reprioritiize", r.ReprioritiizeHandler)
	http.ListenAndServe(":8080", nil)
}
