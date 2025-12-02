package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/evakaiing/go_grpc_microservices/internal/app" 
)

func main() {
	addr := "127.0.0.1:8082"
	aclData := `{
        "logger":    ["/admin.Admin/Logging"],
        "stat":      ["/admin.Admin/Statistics"],
        "biz_user":  ["/biz.Biz/Check", "/biz.Biz/Add"],
        "biz_admin": ["/biz.Biz/*"]
    }`

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
		<-stop
		log.Println("Shutting down...")
		cancel()
	}()

	log.Printf("Starting server at %s", addr)
	if err := app.StartMyMicroservice(ctx, addr, aclData); err != nil {
		log.Fatal(err)
	}
    
    <-ctx.Done()
    log.Println("Server stopped")
}
