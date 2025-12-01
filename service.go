package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	admService "hw7/services/admin"
	pbAdmin "hw7/services/admin/pb"
	bizService "hw7/services/biz"
	pbBiz "hw7/services/biz/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type Server struct {
	admin *admService.AdminManager
	acl   map[string][]string
}

func getClientHost(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if ok {
		return p.Addr.String()
	}
	return "unknown"
}

func (s *Server) checkACL(ctx context.Context, fullMethod string) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "metadata is not provided")
	}

	consumers := md["consumer"]
	if len(consumers) == 0 {
		return "", status.Error(codes.Unauthenticated, "consumers is not provided")
	}

	consumer := consumers[0]
	allowedMethods, exist := s.acl[consumer]
	if !exist {
		return "", status.Error(codes.Unauthenticated, "consumer not found in ACL")
	}

	isAllowed := false
	for _, method := range allowedMethods {
		if method == fullMethod {
			isAllowed = true
			break
		}
		if strings.HasSuffix(method, "/*") {
			prefix := strings.TrimSuffix(method, "*")
			if strings.HasPrefix(fullMethod, prefix) {
				isAllowed = true
				break
			}
		}
	}

	if !isAllowed {
		return "", status.Error(codes.Unauthenticated, "access denied")
	}

	return consumer, nil
}

func (s *Server) logAndStat(ctx context.Context, consumer string, fullMethod string) {
	s.admin.Mu.Lock()
	s.admin.StatByMethod[fullMethod]++
	s.admin.StatByConsumer[consumer]++
	s.admin.Mu.Unlock()

	host := getClientHost(ctx)

	event := &pbAdmin.Event{
		Timestamp: time.Now().Unix(),
		Consumer:  consumer,
		Method:    fullMethod,
		Host:      host,
	}

	s.admin.Mu.RLock()
	for ch := range s.admin.Subscribes {
		select {
		case ch <- event:
		default:
		}
	}
	s.admin.Mu.RUnlock()
}

func (s *Server) Interceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {

	consumer, err := s.checkACL(ctx, info.FullMethod)
	if err != nil {
		return nil, err
	}

	reply, err := handler(ctx, req)

	s.logAndStat(ctx, consumer, info.FullMethod)

	return reply, err
}

func (s *Server) StreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {

	ctx := ss.Context()
	consumer, err := s.checkACL(ctx, info.FullMethod)
	if err != nil {
		return err
	}

	s.logAndStat(ctx, consumer, info.FullMethod)

	return handler(srv, ss)
}

func StartMyMicroservice(ctx context.Context, addr string, acl string) error {
	adminMgr := &admService.AdminManager{
		Subscribes:     make(map[chan *pbAdmin.Event]struct{}),
		StatByMethod:   make(map[string]uint64),
		StatByConsumer: make(map[string]uint64),
	}

	bizMgr := &bizService.BizManager{}

	aclMap := make(map[string][]string)
	if err := json.Unmarshal([]byte(acl), &aclMap); err != nil {
		return err
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalln("cant listen", err)
	}

	srv := Server{
		admin: adminMgr,
		acl:   aclMap,
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(srv.Interceptor),
		grpc.StreamInterceptor(srv.StreamInterceptor),
	)

	pbAdmin.RegisterAdminServer(grpcServer, adminMgr)
	pbBiz.RegisterBizServer(grpcServer, bizMgr)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			fmt.Println("all is down")
		}
	}()

	go func() {
		<-ctx.Done()
		grpcServer.Stop()
	}()

	return nil
}
