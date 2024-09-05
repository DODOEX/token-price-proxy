package shared

import (
	"log"
	"os"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func NewEtcdClient() *clientv3.Client {
	v := os.Getenv("LUFFY_ETCD_CONFIG_ENDPOINTS")
	e := strings.Split(v, " ")
	if len(e) == 0 {
		log.Print("LUFFY_ETCD_CONFIG_ENDPOINTS is empty")
		return nil
	}

	// 创建 etcd 客户端
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   e,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	return client
}
