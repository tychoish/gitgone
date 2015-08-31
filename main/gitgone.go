package main

import (
	"fmt"

	git "github.com/tychoish/gitgone"
)

func main() {
	garenRepo := git.NewWrappedRepository("~/garen/")
	fmt.Printf("\n%+v\n\n", garenRepo)
	fmt.Println(garenRepo.Branch())
}
