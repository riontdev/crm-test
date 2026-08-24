package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func main() {
	url := strings.Replace(os.Getenv("DATABASE_URL"), ":6543", ":5432", 1) + "&default_query_exec_mode=simple_protocol"
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		panic(err)
	}
	_ = cfg
	conn, err := pgx.Connect(context.Background(), url)
	_ = err
	if err != nil {
		panic(err)
	}
	if err != nil {
		panic(err)
	}
	defer conn.Close(context.Background())

	param := uuid.NullUUID{UUID: uuid.MustParse("668312a1-b553-40bd-b7c9-67663cbd016f"), Valid: true}
	cid := uuid.MustParse("c2fbb8f2-6127-42c8-adac-d2efc68ea09f")
	_, err = conn.Exec(context.Background(),
		"UPDATE conversations SET assigned_to=$1, updated_at=now() WHERE id=$2", param, cid)
	fmt.Println("simple-proto UPDATE:", err)

	// y scan de fila con valor
	var got uuid.NullUUID
	err = conn.QueryRow(context.Background(),
		"SELECT assigned_to FROM conversations WHERE id=$1", cid).Scan(&got)
	fmt.Println("simple-proto SCAN:", got.UUID.String(), got.Valid, "| err:", err)
}
