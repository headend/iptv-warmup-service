package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// load config
	defer func() {
		if err := recover(); err != nil {
			log.Println("defer handle")
			fmt.Println(err)

		}

	}()
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
	agentConn.InitializeClient(conf.RPC.Agent.Gateway, conf.RPC.Agent.Port)
	defer agentConn.Client.Close()
	//	connect agent services
	agentClient := agentpb.NewAgentServiceClient(agentConn.Client)
	for {
		msg, err := mq.Consumer.ReadMessage(-1)
		if err != nil {
			log.Printf("Consumer error: %v\n", err)
			log.Print("Retry connect...")
			time.Sleep(10*time.Second)
			continue
		}
		go func() {
			//defer func() {
			//	if err := recover(); err != nil {
			//		fmt.Println(err)
			//
			//	}
			//
			//}()
			log.Print(string(msg.Value))
			var warmupData *warmup.WarmupMessage
			json.Unmarshal(msg.Value, &warmupData)

			switch warmupData.WupType {
			case "interval":
				// khai báo sẵn 1 map chứa thông tin agent được gửi qua đang kết nối
				// agentid : ipcontrol
				var acvieAgent = make(map[string]bool)
				// Chay default neu cac case tren khong match
				for _, newInfo := range warmupData.Data{
					err3 := UpdateAgentStatusOnly(agentClient, newInfo, true)
					if err3 != nil {
						log.Println(err3.Error())
						continue
					}
					acvieAgent[newInfo.IP] = newInfo.Status
				}
				// update active agent done
				// now update agent not in list active
				// get all agent
				agentList, err6 := getAllAgent(agentClient)
				if err6 != nil {
					log.Println(err6.Error())
				} else {
					for _, agent := range agentList {
						// check agent id in list active
						_, ok := acvieAgent[agent.IpControl]
						//log.Printf("value %v", value)
						//log.Printf("ok: %v", ok)
						// Nếu không tồn tại thì cập nhật trạng thái false hết
						if ok {
							continue
						}
						if agent.Status != false {
							newStatus := false
							err8 := updateAgentStatusOnlyByID(agentClient, agent.Id, newStatus)
							if err8 != nil {
								log.Println(err8)
								continue
							}
						}
					}
				}
			case "event":
				if len(warmupData.Data) == 1 {
					err4 := UpdateAgentStatusOnly(agentClient, warmupData.Data[0], warmupData.Data[0].Status)
					if err4 != nil {
						log.Println(err4.Error())
					}
				} else {
					log.Printf("Invalid data input from %s", string(msg.Value))
				}
			default :
				if len(warmupData.Data) == 1 {
					err4 := UpdateAgentStatusOnly(agentClient, warmupData.Data[0], warmupData.Data[0].Status)
					if err4 != nil {
						log.Println(err4.Error())
					}
				} else {
					log.Printf("Invalid data input from %s", string(msg.Value))
				}
			}
			// End switch - case
			time.Sleep(1 * time.Second)
		}()
	}
}

//========================================================================================================

func updateAgentStatusOnlyByID(agentClient agentpb.AgentServiceClient, id int64, newStatus bool) error {
	c, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err8 := (agentClient).UpdateStatus(c, &agentpb.AgentUpdateStatus{
		Id:     id,
		Status: newStatus,
	})
	return err8
}

func getAllAgent(agentClient agentpb.AgentServiceClient) (agentList []*agentpb.Agent, err error) {
	c, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err5 := (agentClient).Gets(c, &agentpb.AgentGetAll{})
	if err5 != nil {
		return nil, err5
	}
	return res.Agents, nil
}

func UpdateAgentStatusOnly(agentClient agentpb.AgentServiceClient, newInfo warmup.WarmupElement, newStatus bool) (err error) {
	var ip = newInfo.IP
	c, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err2 := (agentClient).UpdateStatus(c, &agentpb.AgentUpdateStatus{
		IpControl: ip,
		Status:    newStatus,
	})
	if err2 != nil {
		return err2
	}
	//log.Printf("response: %#v", res.Agents)
	return nil
}
