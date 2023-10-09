package main

import "go_components_record/components/log"

func main() {
	// zap log的使用
	log.InitLogger("")
	log.GetSugarLogInstance().Info("test zap log")
}
