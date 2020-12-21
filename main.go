package main

import (
	"context"
	"encoding/json"
	messagequeue "github.com/headend/share-module/MQ"
	"github.com/headend/share-module/configuration"
	"github.com/headend/share-module/model/warmup"
	"log"
	agentpb "github.com/headend/iptv-agent-service/proto"
	"time"
	myRpc"github.com/headend/share-module/mygrpc/connection"
)


/*
=> Nếu data gửi qua chỉ chứa 1 object thì chỉ cần cập nhật đúng object đó theo status gửi qua
=> Nếu data gửi qua bao gồm nhiều object thì case này cập nhật status là True hết, những host nào không có trong danh sách thì cập nhật là false
Note: Cập nhật ngay và luôn không cần phải gọi DB kiểm tra tồn tại làm gì, hệ thống đâu có nhiu agent kết nối đâu, nên không cần phải chạy multithread làm gì
 */


func main()  {
	// load config
	var conf configuration.Conf
	conf.LoadConf()
	var mq messagequeue.MQ
	mq.InitConsumerByTopic(&conf, conf.MQ.WarmUpTopic)
	defer mq.CloseConsumer()
	if mq.Err != nil {
		log.Print(mq.Err)
	}

	log.Printf("Listen mesage from %s topic\n",conf.MQ.WarmUpTopic)
	var agentConn myRpc.RpcClient
	//try connect to agent
	agentConn.InitializeClient(conf.RPC.Agent.Host, string(conf.RPC.Agent.Port))
	defer agentConn.Client.Close()
	//	connect agent services
	agentClient := agentpb.NewAgentCTLServiceClient(agentConn.Client)
	for {
		msg, err := mq.Consumer.ReadMessage(-1)
		if err != nil {
			log.Printf("Consumer error: %v (%v)\n", err, msg)
			log.Print("Se you again!")
			break
		}
		log.Print(msg.Value)
		var warmupData *warmup.WarmupMessage
		json.Unmarshal(msg.Value, &warmupData)

		switch len(warmupData.Data) {
		case 0:
			log.Printf("No matching data from %s", string(msg.Value))
		case 1:
			err4 := UpdateAgentStatusOnly(agentClient, warmupData.Data[0], warmupData.Data[0].Status)
			if err4 != nil {
				log.Println(err4.Error())
			}
		default:
			// Chay default neu cac case tren khong match
			for _, newInfo := range warmupData.Data{
				err3 := UpdateAgentStatusOnly(agentClient, newInfo, true)
				if err3 != nil {
					log.Println(err3.Error())
				}
			}
			// End switch - case
		}
		time.Sleep(1 * time.Second)
	}
}

func UpdateAgentStatusOnly(agentClient agentpb.AgentCTLServiceClient, newInfo warmup.WarmupElement, newStatus bool) (err error) {
	c, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err2 := (agentClient).UpdateStatus(c, &agentpb.AgentUpdateStatus{
		IpControl: newInfo.IP,
		Status:    newStatus,
	})
	if err2 != nil {
		return err2
	}
	return nil
}
