package main

import (
	"fmt"

	git "github.com/tychoish/gitgone"
	"github.com/tychoish/gitgone/store"
)

func main() {
	garenRepo := git.NewWrappedRepository("~/garen/")
	fmt.Printf("\n%+v\n\n", garenRepo)
	fmt.Println(garenRepo.Branch())

	db, err := store.NewDatabase(".git")
	c := db.Collection("test")
	fmt.Println(db, c, err)
}
