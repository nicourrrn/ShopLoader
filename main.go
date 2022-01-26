package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

func GetRowId(db *sql.DB, selectQuery, insertQuery string, args ...interface{}) int64 {
	row := db.QueryRow(selectQuery, args...)
	var id int64
	err := row.Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Println(err)
		}
		result, err := db.Exec(insertQuery, args...)
		if err != nil {
			if strings.HasPrefix(err.Error(), "Error 1062") {
				return GetRowId(db, selectQuery, insertQuery, args...)
			}
		}
		id, err = result.LastInsertId()
		if err != nil {
			log.Fatalln(err)
		}
	}
	return id
}

func main() {

	pool := NewWorkerPool(4)
	wg := sync.WaitGroup{}
	db, err := sql.Open("mysql", "student:1234@/fullstack_shop")
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()
	wg.Add(pool.Count)
	for i := 0; i < pool.Count; i++ {
		fmt.Println("start")
		go pool.Run(&wg, func(shop Shop) {
			shopTypeId := GetRowId(db, "SELECT id FROM shop_types WHERE name = ?",
				"INSERT INTO shop_types(name) VALUE (?)", shop.Type)
			_, err = db.Exec("INSERT INTO shops VALUE (?, ?, ?, ?, ?, ?)",
				shop.Id, shop.Name, shopTypeId, shop.Image, shop.WorkingHours.Opening,
				shop.WorkingHours.Closing)

			if err != nil {
				log.Println(err)
			}

			for _, prod := range shop.Menu {
				prodTypeId := GetRowId(db, "SELECT id FROM prod_types WHERE name = ?",
					"INSERT INTO prod_types(name) VALUE (?)", prod.Type)
				_, err = db.Exec(
					"INSERT INTO products VALUE (?, ?, ?, ?, ?, ?)",
					prod.Id, prod.Name, prod.Price, prod.Image, prodTypeId, shop.Id)

				if err != nil {
					log.Println(err)
				}

				for _, ing := range prod.Ingredients {
					ingId := GetRowId(db, "SELECT id FROM ingredients WHERE name = ?",
						"INSERT INTO ingredients(name) VALUE (?)", ing)
					_, err = db.Exec("INSERT INTO prod_ingredient VALUE (?, ?)", prod.Id, ingId)

					if err != nil {
						log.Println(err)
					}
				}
			}
		})
	}

	client := http.DefaultClient

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://foodapi.true-tech.php.nixdev.co/suppliers", nil)
	if err != nil {
		log.Fatalln(err)
	}
	res, err := client.Do(req)
	if err != nil {
		log.Fatalf("%v", err)
	}

	suppliersMap := make(map[string][]Shop, 0)
	err = json.NewDecoder(res.Body).Decode(&suppliersMap)
	if err != nil {
		fmt.Println(err)
	}
	res.Body.Close()
	suppliers := make([]Shop, 0)
	suppliers = suppliersMap["suppliers"]
	for i, _ := range suppliers {
		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		req, err = http.NewRequestWithContext(ctx, http.MethodGet,
			"http://foodapi.true-tech.php.nixdev.co/suppliers/"+strconv.Itoa(suppliers[i].Id)+"/menu",
			nil)
		if err != nil {
			log.Fatalln(err)
		}
		res, err = client.Do(req)
		if err != nil {
			log.Fatalf("%v", err)
		}
		menuMap := make(map[string][]Product, 0)
		json.NewDecoder(res.Body).Decode(&menuMap)
		suppliers[i].Menu = menuMap["menu"]
		pool.Sender <- suppliers[i]
		res.Body.Close()
		cancel()
	}
	pool.Stop()
	wg.Wait()

	for {
		time.Sleep(time.Minute)
		for i, sup := range suppliers {
			for j, prod := range sup.Menu {
				ctx, cancel = context.WithTimeout(context.Background(), time.Second)
				req, err = http.NewRequestWithContext(ctx, http.MethodGet,
					"http://foodapi.true-tech.php.nixdev.co/suppliers/"+
						strconv.Itoa(sup.Id)+"/menu/"+strconv.Itoa(prod.Id),
					nil)
				res, err = client.Do(req)
				if err != nil {
					log.Println("update error: " + err.Error())
				}
				var p Product
				err = json.NewDecoder(res.Body).Decode(&p)
				if err != nil {
					log.Println("update error: " + err.Error())
				}
				if p.Price != prod.Price {
					_, err = db.Exec("UPDATE fullstack_shop.products SET price = ? WHERE id = ?", p.Price, p.Id)
					if err != nil {
						log.Println(err)
					}
					fmt.Println(p.Name, " edit from price", prod.Price, " to ", p.Price)
					suppliers[i].Menu[j].Price = p.Price
				} else {
					fmt.Println(p.Name, " not edit with price", p.Price)
				}
				res.Body.Close()
				cancel()
			}
		}
	}
}
