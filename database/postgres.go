package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// PostgresConfig конфиг для postgres
type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

// GetDatabase получаем коннекшн с базой
func GetDatabase(cfg PostgresConfig) (*sql.DB, error) {
	postgresqlDbInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)
	return sql.Open("postgres", postgresqlDbInfo)
}

// FindGoods ищем товары
func FindGoods(db *sql.DB, limit, offset int) (payload json.RawMessage, err error) {
	rows, err := db.Query("select * from test_issue.goods order by id limit $1 offset $2", limit, offset)
	if err != nil {
		return nil, err
	}
	rowTotal, err := db.Query("select count(*) from test_issue.goods")
	if err != nil {
		return nil, err
	}
	rowRemoved, err := db.Query("select count(*) from test_issue.goods where removed = true")
	if err != nil {
		return nil, err
	}
	defer func() {
		rows.Close()
		rowTotal.Close()
		rowRemoved.Close()
	}()

	goods := make([]Good, 0, limit)
	for rows.Next() {
		good := Good{}
		err = rows.Scan(&good.ID, &good.ProjectID, &good.Name, &good.Description, &good.Priority, &good.Removed, &good.CreatedAt)
		if err != nil {
			return nil, err
		}
		goods = append(goods, good)
	}

	total := 0
	rowTotal.Next()
	err = rowTotal.Scan(&total)
	if err != nil {
		return nil, err
	}

	removed := 0
	rowRemoved.Next()
	err = rowRemoved.Scan(&removed)
	if err != nil {
		return nil, err
	}

	goodsResponse := GoodsResponse{
		Meta: Meta{
			Total:   total,
			Removed: removed,
			Limit:   limit,
			Offset:  offset,
		},
		Goods: goods,
	}

	out, err := json.Marshal(goodsResponse)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GoodsResponse структура ответа для get-запроса
type GoodsResponse struct {
	Meta  Meta   `json:"meta"`
	Goods []Good `json:"goods"`
}

// Meta метаданные для ответа
type Meta struct {
	Total   int `json:"total"`
	Removed int `json:"removed"`
	Limit   int `json:"limit"`
	Offset  int `json:"offset"`
}

// Good структура товара
type Good struct {
	ID          int       `json:"id"`
	ProjectID   int       `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	Removed     bool      `json:"removed"`
	CreatedAt   time.Time `json:"createdAt"`
}
