package updater

import "fmt"

var Version = "dev"

func PrintVersion() {
	fmt.Printf("websz version %s\n", Version)
}
