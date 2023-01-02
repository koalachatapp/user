package port

import (
	"sync"

	"github.com/Shopify/sarama"
)

// Worker struct has fields for synchronization (WaitGroup and Mutex),
// task processing (Worker channel), sending data to Kafka (Send2kafka channel),
// and a Sarama async producer (Prod).
type Worker struct {
	Wg         sync.WaitGroup
	Wg2        sync.WaitGroup
	Lock       sync.Mutex
	Worker     chan map[uint8]interface{}
	Send2kafka chan string
	Prod       sarama.SyncProducer
}
