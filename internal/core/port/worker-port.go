package port

import (
	"log"
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
	Prod       *sarama.AsyncProducer
}

type Producerr struct {
}

func (w *Worker) RunProduserWorker() {
	for i := 0; i < 2; i++ {
		go func(msg <-chan string, wg *sync.WaitGroup) {
			for msg := range w.Send2kafka {
				// w.Lock.Lock()
				log.Println("Sending to Kafka : ", msg)
				err := (*w.Prod).BeginTxn()
				if err != nil {
					log.Println(err)
				}
				// if (*w.Prod).IsTransactional() {
				// 	log.Println("transaction is on progress")
				// 	continue
				// }
				var suc int = 0
				select {
				case (*w.Prod).Input() <- &sarama.ProducerMessage{
					Topic: "UsersearchTopic",
					Value: sarama.StringEncoder(msg),
				}:
					suc++
				case err := <-(*w.Prod).Errors():
					log.Println(err.Err)
				}
				if err := (*w.Prod).CommitTxn(); err != nil {
					log.Printf("Producer: unable to commit txn %s\n", err)
					for {
						if (*w.Prod).TxnStatus()&sarama.ProducerTxnFlagFatalError != 0 {
							// fatal error. need to recreate producer.
							log.Printf("Producer: producer is in a fatal state, need to recreate it")
							break
						}
						// If producer is in abortable state, try to abort current transaction.
						if (*w.Prod).TxnStatus()&sarama.ProducerTxnFlagAbortableError != 0 {
							err = (*w.Prod).AbortTxn()
							if err != nil {
								// If an error occured just retry it.
								log.Printf("Producer: unable to abort transaction: %+v", err)
								continue
							}
							break
						}
						// if not you can retry
						err = (*w.Prod).CommitTxn()
						if err != nil {
							log.Printf("Producer: unable to commit txn %s\n", err)
							continue
						}
					}
				}
				if (*w.Prod).TxnStatus()&sarama.ProducerTxnFlagInError != 0 {
					// Try to close it
					_ = (*w.Prod).Close()
					return
				}
				log.Println("data sended to kafka ", suc)
				w.Wg2.Done()
				// w.Lock.Unlock()

			}
		}(w.Send2kafka, &w.Wg2)
	}

}
