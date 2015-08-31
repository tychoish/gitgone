package main

import (
	"fmt"

	git "github.com/tychoish/gitgone"
)

func main() {
	garenRepo := git.NewRepository("~/garen/")
	fmt.Printf("\n%+v\n\n", garenRepo)
}
