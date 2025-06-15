package handler

import (
	"database/sql"
	"log"
	"main/database"
	"net/http"
	"net/url"
	"strconv"
)

// RestHandler структура для обработчика запросов
type RestHandler struct {
	DataBase *sql.DB
}

// NewRestHandler получаем новый обработчик запросов
func NewRestHandler(db *sql.DB) RestHandler {
	return RestHandler{
		DataBase: db,
	}
}

// GetHandler обрабочик get-запроса
func (rh RestHandler) GetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	limit, offset, err := getLimitAndOffset(r.URL.Query())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}

	payload, err := database.FindGoods(rh.DataBase, limit, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		log.Print(err)
		return
	}
	w.WriteHeader(200)
	w.Write(payload)
}

// PostHandler обрабочик post-запроса
func (rh RestHandler) PostHandler(w http.ResponseWriter, r *http.Request) {
}

// getLimitAndOffset получаем лимит и отступ для sql
func getLimitAndOffset(params url.Values) (limit, offset int, err error) {
	l := params.Get("limit")
	if l == "" {
		l = "10"
	}
	limit, err = strconv.Atoi(l)
	if err != nil {
		return 0, 0, err
	}

	o := params.Get("offset")
	if o == "" {
		o = "1"
	}
	offset, err = strconv.Atoi(o)
	if err != nil {
		return 0, 0, err
	}
	return limit, offset, err
}
