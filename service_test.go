//nolint:typecheck
package main

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	status "google.golang.org/grpc/status"

	    adminPb "hw7/services/admin/pb"
    bizPb "hw7/services/biz/pb"
)

const (
	// Какой адрес-порт слушать серверу
	listenAddr string = "127.0.0.1:8082"

	// Кого по каким методам пускать
	ACLData string = `{
    "logger":    ["/pbAdmin.Admin/Logging"],
    "stat":      ["/pbAdmin.Admin/Statistics"],
    "biz_user":  ["/pbBiz.Biz/Check", "/pbBiz.Biz/Add"],
    "biz_admin": ["/pbBiz.Biz/*"]
}`
)

// Чтобы не было сюрпризов когда где-то не успела переключиться горутина и не успело что-то стартовать
func wait(amount int) {
	time.Sleep(time.Duration(amount) * 10 * time.Millisecond)
}

// Утилитарная функция для коннекта к серверу
func getGrpcConn(t *testing.T) *grpc.ClientConn {
	grpcClient, err := grpc.NewClient(
		listenAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("cant connect to grpc: %v", err)
	}
	return grpcClient
}

// Получаем контекст с нужными метаданными для ACL
func getConsumerCtx(consumerName string) context.Context {
	// ctx, _ := context.WithTimeout(context.Background(), time.Second)
	ctx := context.Background()
	md := metadata.Pairs(
		"consumer", consumerName,
	)
	return metadata.NewOutgoingContext(ctx, md)
}

// Старт-стоп сервера
func TestServerStartStop(t *testing.T) {
	ctx, finish := context.WithCancel(context.Background())
	err := StartMyMicroservice(ctx, listenAddr, ACLData)
	if err != nil {
		t.Fatalf("cant start server initial: %v", err)
	}
	wait(1)
	finish() // При вызове этой функции ваш сервер должен остановиться и освободить порт
	wait(1)

	// Теперь проверим, что вы освободили порт и мы можем стартовать сервер ещё раз
	ctx, finish = context.WithCancel(context.Background())
	err = StartMyMicroservice(ctx, listenAddr, ACLData)
	if err != nil {
		t.Fatalf("cant start server again: %v", err)
	}
	wait(1)
	finish()
	wait(1)
}

// У вас наверняка будет что-то выполняться в отдельных горутинах.
// Этим тестом мы проверяем, что вы останавливаете все горутины, которые у вас были, и нет утечек.
// Некоторый запас (goroutinesPerTwoIterations*5) остаётся на случай рантайм горутин
func TestServerLeak(t *testing.T) {
	// return
	goroutinesStart := runtime.NumGoroutine()
	TestServerStartStop(t)
	goroutinesPerTwoIterations := runtime.NumGoroutine() - goroutinesStart

	goroutinesStart = runtime.NumGoroutine()
	goroutinesStat := []int{}
	for i := 0; i <= 25; i++ {
		TestServerStartStop(t)
		goroutinesStat = append(goroutinesStat, runtime.NumGoroutine())
	}
	goroutinesPerFiftyIterations := runtime.NumGoroutine() - goroutinesStart
	if goroutinesPerFiftyIterations > goroutinesPerTwoIterations*5 {
		t.Fatalf("looks like you have goroutines leak: %+v", goroutinesStat)
	}
}

// ACL (права на методы доступа) парсится корректно
func TestACLParseError(t *testing.T) {
	// finish'а тут нет потому что стартовать у вас ничего не должно если не получилось распаковать ACL
	err := StartMyMicroservice(context.Background(), listenAddr, "{.;")
	if err == nil {
		t.Fatalf("expacted error on bad acl json, have nil")
	}
}

// ACL (права на методы доступа) работает корректно
func TestACL(t *testing.T) {
	wait(1)
	ctx, finish := context.WithCancel(context.Background())
	err := StartMyMicroservice(ctx, listenAddr, ACLData)
	if err != nil {
		t.Fatalf("cant start server initial: %v", err)
	}
	wait(1)
	defer func() {
		finish()
		wait(1)
	}()

	conn := getGrpcConn(t)
	defer conn.Close()

	biz := bizPb.NewBizClient(conn)
	adm := adminPb.NewAdminClient(conn)

	for idx, ctx := range []context.Context{
		context.Background(),       // Нет поля для ACL
		getConsumerCtx("unknown"),  // Поле есть, неизвестный консюмер
		getConsumerCtx("biz_user"), // Поле есть, нет доступа
	} {
		_, err = biz.Test(ctx, &bizPb.Nothing{})
		if err == nil {
			t.Fatalf("[%d] ACL fail: expected err on disallowed method", idx)
		} else if code := status.Code(err); code != codes.Unauthenticated {
			t.Fatalf("[%d] ACL fail: expected Unauthenticated code, got %v", idx, code)
		}
	}

	// Есть доступ
	_, err = biz.Check(getConsumerCtx("biz_user"), &bizPb.Nothing{})
	if err != nil {
		t.Fatalf("ACL fail: unexpected error: %v", err)
	}
	_, err = biz.Check(getConsumerCtx("biz_admin"), &bizPb.Nothing{})
	if err != nil {
		t.Fatalf("ACL fail: unexpected error: %v", err)
	}
	_, err = biz.Test(getConsumerCtx("biz_admin"), &bizPb.Nothing{})
	if err != nil {
		t.Fatalf("ACL fail: unexpected error: %v", err)
	}

	// ACL на методах, которые возвращают поток данных
	logger, err := adm.Logging(getConsumerCtx("unknown"), &adminPb.Nothing{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = logger.Recv()
	if err == nil {
		t.Fatalf("ACL fail: expected err on disallowed method")
	} else if code := status.Code(err); code != codes.Unauthenticated {
		t.Fatalf("ACL fail: expected Unauthenticated code, got %v", code)
	}
}

func TestLogging(t *testing.T) {
	ctx, finish := context.WithCancel(context.Background())
	err := StartMyMicroservice(ctx, listenAddr, ACLData)
	if err != nil {
		t.Fatalf("cant start server initial: %v", err)
	}
	wait(1)
	defer func() {
		finish()
		wait(1)
	}()

	conn := getGrpcConn(t)
	defer conn.Close()

	biz := bizPb.NewBizClient(conn)
	adm := adminPb.NewAdminClient(conn)

	logStream1, err := adm.Logging(getConsumerCtx("logger"), &adminPb.Nothing{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1 * time.Millisecond)

	logStream2, err := adm.Logging(getConsumerCtx("logger"), &adminPb.Nothing{})
	if err != nil {
		t.Fatal(err)
	}

	logData1 := []*adminPb.Event{}
	logData2 := []*adminPb.Event{}

	wait(1)

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
			fmt.Println("looks like you dont send anything to log stream in 3 sec")
			t.Errorf("looks like you dont send anything to log stream in 3 sec")
		}
	}()

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 4; i++ {
			evt, errTmp := logStream1.Recv()
			// log.Println("logger 1", evt, errTmp)
			if errTmp != nil {
				t.Errorf("unexpected error: %v, awaiting event", errTmp)
				return
			}
			if !strings.HasPrefix(evt.GetHost(), "127.0.0.1:") || evt.GetHost() == listenAddr {
				t.Errorf("bad host: %v", evt.GetHost())
				return
			}
			// Это грязный хак.
			// protobuf добавляет к структуре свои поля, которые не видны при приведении к строке и при reflect.DeepEqual.
			// Поэтому берем не оригинал сообщения, а только нужные значения
			logData1 = append(logData1, &adminPb.Event{Consumer: evt.Consumer, Method: evt.Method})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			evt, errTmp := logStream2.Recv()
			// log.Println("logger 2", evt, errTmp)
			if errTmp != nil {
				t.Errorf("unexpected error: %v, awaiting event", errTmp)
				return
			}
			if !strings.HasPrefix(evt.GetHost(), "127.0.0.1:") || evt.GetHost() == listenAddr {
				t.Errorf("bad host: %v", evt.GetHost())
				return
			}
			// Это грязный хак.
			// protobuf добавляет к структуре свои поля, которые не видны при приведении к строке и при reflect.DeepEqual.
			// Поэтому берем не оригинал сообщения, а только нужные значения
			logData2 = append(logData2, &adminPb.Event{Consumer: evt.Consumer, Method: evt.Method})
		}
	}()

	_, err = biz.Check(getConsumerCtx("biz_user"), &bizPb.Nothing{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)

	_, err = biz.Check(getConsumerCtx("biz_admin"), &bizPb.Nothing{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)

	_, err = biz.Test(getConsumerCtx("biz_admin"), &bizPb.Nothing{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)

	wg.Wait()

	expectedLogData1 := []*adminPb.Event{
		{Consumer: "logger", Method: "/pbAdmin.Admin/Logging"},
		{Consumer: "biz_user", Method: "/pbBiz.Biz/Check"},
		{Consumer: "biz_admin", Method: "/pbBiz.Biz/Check"},
		{Consumer: "biz_admin", Method: "/pbBiz.Biz/Test"},
	}
	expectedLogData2 := []*adminPb.Event{
		{Consumer: "biz_user", Method: "/pbBiz.Biz/Check"},
		{Consumer: "biz_admin", Method: "/pbBiz.Biz/Check"},
		{Consumer: "biz_admin", Method: "/pbBiz.Biz/Test"},
	}

	if !reflect.DeepEqual(logData1, expectedLogData1) {
		t.Fatalf("logs1 dont match\nhave %+v\nwant %+v", logData1, expectedLogData1)
	}
	if !reflect.DeepEqual(logData2, expectedLogData2) {
		t.Fatalf("logs2 dont match\nhave %+v\nwant %+v", logData2, expectedLogData2)
	}
}
func TestStat(t *testing.T) {
	ctx, finish := context.WithCancel(context.Background())
	err := StartMyMicroservice(ctx, listenAddr, ACLData)
	if err != nil {
		t.Fatalf("cant start server initial: %v", err)
	}
	wait(1)
	defer func() {
		finish()
		wait(2)
	}()

	conn := getGrpcConn(t)
	defer conn.Close()

	biz := bizPb.NewBizClient(conn)
	adm := adminPb.NewAdminClient(conn)

	// Первый стрим статистики (раз в 2 сек)
	statStream1, err := adm.Statistics(getConsumerCtx("stat"), &adminPb.StatInterval{IntervalSeconds: 2})
	if err != nil {
		t.Fatal(err)
	}
	wait(1)
	// Второй стрим статистики (раз в 3 сек)
	statStream2, err := adm.Statistics(getConsumerCtx("stat"), &adminPb.StatInterval{IntervalSeconds: 3})
	if err != nil {
		t.Fatal(err)
	}

	mu := &sync.Mutex{}
	stat1 := &adminPb.Stat{}
	stat2 := &adminPb.Stat{}

	wg := &sync.WaitGroup{}
	wg.Add(2)
	
	// Читаем первый стрим
	go func() {
		for {
			stat, errTmp := statStream1.Recv()
			if errTmp != nil && errTmp != io.EOF {
				return
			} else if errTmp == io.EOF {
				break
			}
			mu.Lock()
			stat1 = &adminPb.Stat{
				ByMethod:   stat.ByMethod,
				ByConsumer: stat.ByConsumer,
			}
			mu.Unlock()
		}
	}()

	// Читаем второй стрим
	go func() {
		for {
			stat, errTmp := statStream2.Recv()
			if errTmp != nil && errTmp != io.EOF {
				return
			} else if errTmp == io.EOF {
				break
			}
			mu.Lock()
			stat2 = &adminPb.Stat{
				ByMethod:   stat.ByMethod,
				ByConsumer: stat.ByConsumer,
			}
			mu.Unlock()
		}
	}()

	wait(1)

	// Делаем вызовы методов
	_, err = biz.Check(getConsumerCtx("biz_user"), &bizPb.Nothing{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = biz.Add(getConsumerCtx("biz_user"), &bizPb.Nothing{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = biz.Test(getConsumerCtx("biz_admin"), &bizPb.Nothing{})
	if err != nil {
		t.Fatal(err)
	}

	wait(200) // Ждем 2 сек (чтобы пришел апдейт в statStream1)

	// Ожидание 1: После первых 3 вызовов + 2 вызова Statistics
	expectedStat1 := &adminPb.Stat{
		ByMethod: map[string]uint64{
			"/pbBiz.Biz/Check":          1,
			"/pbBiz.Biz/Add":            1,
			"/pbBiz.Biz/Test":           1,
			"/pbAdmin.Admin/Statistics": 2, // Было 2 вызова adm.Statistics
		},
		ByConsumer: map[string]uint64{
			"biz_user":  2, // Check + Add
			"biz_admin": 1, // Test
			"stat":      2, // Два стрима статистики
		},
	}

	mu.Lock()
	if !reflect.DeepEqual(stat1, expectedStat1) {
		t.Fatalf("stat1-1 dont match\nhave %+v\nwant %+v", stat1, expectedStat1)
	}
	mu.Unlock()

	// Делаем еще один вызов
	_, err = biz.Add(getConsumerCtx("biz_admin"), &bizPb.Nothing{})
	if err != nil {
		t.Fatal(err)
	}

	wait(220) // Ждем еще 2+ сек (чтобы пришел апдейт и в statStream1, и в statStream2)

	// Ожидание 2: Накопленная статистика после 4-го вызова
	expectedStat1 = &adminPb.Stat{
		Timestamp: 0,
		ByMethod: map[string]uint64{
			"/pbBiz.Biz/Check":          1,
			"/pbBiz.Biz/Add":            2, // Стало 2 (было 1)
			"/pbBiz.Biz/Test":           1,
			"/pbAdmin.Admin/Statistics": 2,
		},
		ByConsumer: map[string]uint64{
			"biz_user":  2,
			"biz_admin": 2, // Стало 2 (было 1)
			"stat":      2,
		},
	}
	
	// stat2 (интервал 3 сек) должен показать то же самое, так как статистика не сбрасывается
	expectedStat2 := &adminPb.Stat{
		Timestamp: 0,
		ByMethod: map[string]uint64{
			"/pbBiz.Biz/Check":          1,
			"/pbBiz.Biz/Add":            2,
			"/pbBiz.Biz/Test":           1,
			"/pbAdmin.Admin/Statistics": 2, // Тут тоже должны быть эти вызовы
		},
		ByConsumer: map[string]uint64{
			"biz_user":  2,
			"biz_admin": 2,
			"stat":      2,
		},
	}

	mu.Lock()
	if !reflect.DeepEqual(stat1, expectedStat1) {
		t.Fatalf("stat1-2 dont match\nhave %+v\nwant %+v", stat1, expectedStat1)
	}
	
	// Проверяем stat2
	// Внимание: stat2 обновляется раз в 3 секунды. 
	// Мы ждали wait(200) + wait(220) = ~4.2 секунды.
	// Этого достаточно для первого тика (3 сек), но может быть на грани для второго.
	// Если stat2 не совпадает, возможно стоит увеличить время ожидания или проверить только наличие данных.
	if !reflect.DeepEqual(stat2, expectedStat2) {
		t.Fatalf("stat2 dont match\nhave %+v\nwant %+v", stat2, expectedStat2)
	}
	mu.Unlock()

	finish()
}
