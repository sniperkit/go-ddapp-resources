package resolvers

import (
	"context"
	"fmt"

	"github.com/distributed-vision/go-resources/resolvers/infokeys"
	lru "github.com/hashicorp/golang-lru"
)

type Cache interface {
	Get(key interface{}) (interface{}, bool)
	Peek(key interface{}) (interface{}, bool)
	Keys() []interface{}
	Add(key interface{}, value interface{})
	Remove(key interface{})
	Len() int
	Purge()
}

type arcCache struct {
	*lru.ARCCache
}

type CachingResolver struct {
	resolverInfo ResolverInfo
	cache        Cache
}

func (this *CachingResolver) Cache() Cache {
	return this.cache
}

func NewCachingResolver(baseInfo ResolverInfo, cacheSize int) (*CachingResolver, error) {
	if baseInfo == nil {
		return nil, fmt.Errorf("base resolver info must be defined")
	}

	if cacheSizeValue := baseInfo.Value(infokeys.CACHE_SIZE); cacheSizeValue != nil {
		if infoCacheSize, ok := cacheSizeValue.(int); ok {
			cacheSize = infoCacheSize
		}
	}

	cache, err := lru.NewARC(cacheSize)

	if err != nil {
		return nil, err
	}

	return &CachingResolver{baseInfo.DerivedCopy(), arcCache{cache}}, nil
}

func (this *CachingResolver) ResolverInfo() ResolverInfo {
	return this.resolverInfo
}

func (this *CachingResolver) Get(resolutionContext context.Context, selector Selector) (interface{}, error) {

	var key string

	switch selector.Key().(type) {
	case string:
		key = selector.Key().(string)
	case []byte:
		key = string(selector.Key().([]byte))
	default:
		key = fmt.Sprintf("%v", selector.Key())
	}

	entity, ok := this.cache.Get(key)

	//fmt.Printf("key=%v, entity: %v\n", key, entity)
	if ok {
		if selector.Test(entity) {
			return entity, nil
		}
	}

	for _, key := range this.cache.Keys() {
		if entity, ok = this.cache.Peek(key); ok {
			if selector.Test(entity) {
				return entity, nil
			}
		}
	}

	return nil, fmt.Errorf("Can't resolve entity for %v", selector)
}

func (this *CachingResolver) Resolve(resolutionContext context.Context, selector Selector) (chan interface{}, chan error) {
	cres, cerr := make(chan interface{}), make(chan error)

	go func() {
		entity, err := this.Get(resolutionContext, selector)

		if err != nil {
			cerr <- err
		} else {
			cres <- entity
		}

		close(cres)
		close(cerr)
	}()

	return cres, cerr
}
