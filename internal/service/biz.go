package service

import (
	"context"
	pbBiz "github.com/evakaiing/go_grpc_microservices/pkg/api/biz"
)

type BizManager struct {
	pbBiz.UnimplementedBizServer
}

func (bm *BizManager) Check(context.Context, *pbBiz.Nothing) (*pbBiz.Nothing, error) {
	return &pbBiz.Nothing{}, nil
}
func (bm *BizManager) Add(context.Context, *pbBiz.Nothing) (*pbBiz.Nothing, error) {
	return &pbBiz.Nothing{}, nil
}
func (bm *BizManager) Test(context.Context, *pbBiz.Nothing) (*pbBiz.Nothing, error) {
	return &pbBiz.Nothing{}, nil
}
