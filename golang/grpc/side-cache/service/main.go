package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/joho/godotenv"
	cachepb "github.com/rigazilla/gingersnap-cloud-api-examples/golang/grpc/side-cache/service/gingersnap-cloud-api/service/cache/v1alpha"
	cachepbv1alpha2 "github.com/rigazilla/gingersnap-cloud-api-examples/golang/grpc/side-cache/service/gingersnap-cloud-api/service/cache/v1alpha2"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

//go:generate protoc --proto_path=../../../../gingersnap-cloud-api service/cache/v1alpha/cache.proto --go-grpc_out=. --go_out=.
//go:generate protoc --proto_path=../../../../gingersnap-cloud-api --grpc-gateway_out=logtostderr=true:. service/cache/v1alpha/cache.proto
//go:generate protoc --proto_path=../../../../gingersnap-cloud-api service/cache/v1alpha2/cache.proto --go-grpc_out=. --go_out=.
//go:generate protoc --proto_path=../../../../gingersnap-cloud-api --grpc-gateway_out=logtostderr=true:. service/cache/v1alpha2/cache.proto
type cacheServer struct {
	cachepb.UnimplementedCacheServiceServer
}

type cacheServervalpha2 struct {
	cachepbv1alpha2.UnimplementedCacheServiceServer
}

func (s *cacheServer) Get(ctx context.Context, k *cachepb.Key) (*cachepb.Value, error) {

	retVal := &cachepb.Value{Value: append([]byte{'h', 'e', 'l', 'l', 'o', ' '}, k.Key...)}
	fmt.Printf("Called Get on server")
	return retVal, nil
}

func (s *cacheServervalpha2) Get(ctx context.Context, k *cachepbv1alpha2.Key) (*cachepbv1alpha2.Value, error) {
	retVal := &cachepbv1alpha2.Value{Value: append([]byte("Ciao "), k.Key...)}
	fmt.Printf("v1alpha2: Called Get on server")
	return retVal, nil
}

func main() {

	if os.Getenv("GRPC_SERVER_PORT") == "" {
		e := godotenv.Load() //Load .env file for local environment
		if e != nil {
			fmt.Println(e)
		}
	}
	// Create a listener on TCP port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", os.Getenv("GRPC_SERVER_PORT")))
	if err != nil {
		log.Fatalln("Failed to listen:", err)
	}

	// Create a gRPC server object
	s := grpc.NewServer()
	// Attach the User service to the server
	cachepb.RegisterCacheServiceServer(s, &cacheServer{})
	cachepbv1alpha2.RegisterCacheServiceServer(s, &cacheServervalpha2{})

	// Serve gRPC server
	log.Printf("Serving gRPC on %s:%s", os.Getenv("SERVER_HOST"), os.Getenv("GRPC_SERVER_PORT"))
	go func() {
		log.Fatalln(s.Serve(lis))
	}()

	maxMsgSize := 1024 * 1024 * 20
	// Create a client connection to the gRPC server we just started
	// This is where the gRPC-Gateway proxies the requests
	conn, err := grpc.DialContext(
		context.Background(),
		fmt.Sprintf("%s:%s", os.Getenv("SERVER_HOST"), os.Getenv("GRPC_SERVER_PORT")),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize), grpc.MaxCallSendMsgSize(maxMsgSize)),
	)
	if err != nil {
		log.Fatalln("Failed to dial server:", err)
	}

	gwmux := runtime.NewServeMux()
	errNewServ := cachepb.RegisterCacheServiceHandler(context.Background(), gwmux, conn)
	if errNewServ != nil {
		log.Fatalln("Failed to register gateway:", errNewServ)
	}

	errNewServ = cachepbv1alpha2.RegisterCacheServiceHandler(context.Background(), gwmux, conn)
	if errNewServ != nil {
		log.Fatalln("Failed to register gateway:", errNewServ)
	}

	gwServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", os.Getenv("GRPC_GATEWAY_SERVER_PORT")),
		Handler: cors(gwmux),
	}

	log.Printf("Serving gRPC-Gateway on %s:%s", os.Getenv("SERVER_HOST"),
		os.Getenv("GRPC_GATEWAY_SERVER_PORT"))
	log.Fatalln(gwServer.ListenAndServe())
}

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if AllowedOrigin(r.Header.Get("Origin")) {
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE")
			w.Header().Set("Access-Control-Allow-Headers",
				"Accept, Content-Type, Content-Length, Accept-Encoding, Authorization, ResponseType")
		}
		if r.Method == "OPTIONS" {
			return
		}
		h.ServeHTTP(w, r)
	})
}

func AllowedOrigin(origin string) bool {
	if viper.GetString("cors") == "*" {
		return true
	}
	if matched, _ := regexp.MatchString(viper.GetString("cors"), origin); matched {
		return true
	}
	return false
}
