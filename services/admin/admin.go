package service

import (
	pbAdmin "hw7/services/admin/pb"
	"sync"
	"time"

	"google.golang.org/grpc"
)

type AdminManager struct {
	pbAdmin.UnimplementedAdminServer
	Subscribes map[chan *pbAdmin.Event]struct{}
	Mu         sync.RWMutex

	StatByMethod   map[string]uint64
	StatByConsumer map[string]uint64
}

func (adm *AdminManager) Logging(in *pbAdmin.Nothing, inStream grpc.ServerStreamingServer[pbAdmin.Event]) error {
	msgCh := make(chan *pbAdmin.Event, 10)

	adm.Mu.Lock()
	adm.Subscribes[msgCh] = struct{}{}
	adm.Mu.Unlock()

	defer func() {
		adm.Mu.Lock()
		delete(adm.Subscribes, msgCh)
		adm.Mu.Unlock()
		close(msgCh)
	}()

	for {
		select {
		case event := <-msgCh:
			err := inStream.Send(event)
			if err != nil {
				return err
			}
		case <-inStream.Context().Done():
			return inStream.Context().Err()
		}
	}
}

func (adm *AdminManager) Statistics(in *pbAdmin.StatInterval, inStream grpc.ServerStreamingServer[pbAdmin.Stat]) error {
	ticker := time.NewTicker(time.Duration(in.IntervalSeconds) * time.Second)

	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			adm.Mu.Lock()

			stat := &pbAdmin.Stat{
				Timestamp:  time.Now().Unix(),
				ByMethod:   make(map[string]uint64),
				ByConsumer: make(map[string]uint64),
			}

			for k, v := range adm.StatByMethod {
				stat.ByMethod[k] = v
			}

			for k, v := range adm.StatByConsumer {
				stat.ByConsumer[k] = v
			}

			adm.Mu.Unlock()

			if err := inStream.Send(stat); err != nil {
				return err
			}
		case <-inStream.Context().Done():
			return inStream.Context().Err()
		}

	}
}
