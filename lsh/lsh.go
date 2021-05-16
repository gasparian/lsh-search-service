package lsh

import (
	"errors"
	"github.com/gasparian/lsh-search-go/store"
	"sync"
)

var (
	DistanceErr = errors.New("Distance can't be calculated")
)

// Record holds vector and it's unique identifier generated by `user`
type Record struct {
	ID  string
	Vec []float64
}

// Metric holds implementation of needed distance metric
type Metric interface {
	GetDist(l, r []float64) float64
}

// LshConfig ...
type LshConfig struct {
	mx            *sync.RWMutex
	DistanceThrsh float64
	MaxNN         int
	BatchSize     int
}

// Config holds all needed constants for creating the Hasher instance
type Config struct {
	LshConfig
	HasherConfig
	Mean []float64
	Std  []float64
}

// LSHIndex holds buckets with vectors and hasher instance
type LSHIndex struct {
	config         LshConfig
	index          store.Store
	hasher         *Hasher
	distanceMetric Metric
}

// New creates new instance of hasher and index, where generated hashes will be stored
func NewLsh(config Config, store store.Store, metric Metric) (*LSHIndex, error) {
	hasher := NewHasher(
		config.HasherConfig,
	)
	err := hasher.generate(config.Mean, config.Std)
	if err != nil {
		return nil, err
	}
	config.LshConfig.mx = new(sync.RWMutex)
	return &LSHIndex{
		config:         config.LshConfig,
		hasher:         hasher,
		index:          store,
		distanceMetric: metric,
	}, nil
}

// Train fills new search index with vectors
func (lsh *LSHIndex) Train(records []Record) error {
	err := lsh.index.Clear()
	if err != nil {
		return err
	}
	lsh.config.mx.RLock()
	batchSize := lsh.config.BatchSize
	lsh.config.mx.RUnlock()

	wg := sync.WaitGroup{}
	wg.Add(len(records)/batchSize + len(records)%batchSize)
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}
		go func(batch []Record, wg *sync.WaitGroup) {
			defer wg.Done()
			for _, rec := range batch {
				hashes := lsh.hasher.getHashes(rec.Vec)
				lsh.index.SetVector(rec.ID, rec.Vec)
				for perm, hash := range hashes {
					lsh.index.SetHash(perm, hash, rec.ID)
				}
			}
		}(records[i:end], &wg)
	}
	wg.Wait()
	return nil
}

// Search returns NNs for the query point
func (lsh *LSHIndex) Search(query []float64) ([]Record, error) {
	hashes := lsh.hasher.getHashes(query)
	lsh.config.mx.RLock()
	config := lsh.config
	lsh.config.mx.RUnlock()

	closestSet := make(map[string]bool)
	closest := make([]Record, 0)
	for perm, hash := range hashes {
		if len(closest) >= config.MaxNN {
			break
		}
		iter, err := lsh.index.GetHashIterator(perm, hash)
		if err != nil {
			continue // NOTE: it's normal when we couldn't find bucket for the query point
		}
		for {
			if len(closest) >= config.MaxNN {
				break
			}
			id, opened := iter.Next()
			if !opened {
				break
			}

			if closestSet[id] {
				continue
			}
			vec, err := lsh.index.GetVector(id)
			if err != nil {
				return nil, err
			}
			dist := lsh.distanceMetric.GetDist(vec, query)
			if dist <= config.DistanceThrsh {
				closestSet[id] = true
				closest = append(closest, Record{ID: id, Vec: vec})
			}
		}
	}
	return closest, nil
}

// DumpHasher serializes hasher
func (lsh *LSHIndex) DumpHasher() ([]byte, error) {
	return lsh.hasher.dump()
}

// LoadHasher fills hasher from byte array
func (lsh *LSHIndex) LoadHasher(inp []byte) error {
	return lsh.hasher.load(inp)
}
