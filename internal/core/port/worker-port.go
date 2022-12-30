package port

import "sync"

type Worker struct {
	Wg     sync.WaitGroup
	Worker chan func() error
}
