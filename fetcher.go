package cache

import (
	"fmt"
)

const (
	errInvalidKey = "type:[invalid_key] key:[%s]"
)

type callbackFunc func(key string) (interface{}, error)

type fetcher struct {
	cb        map[string]callbackFunc
	prefixLen int
}

func NewFetcher(prefixLen int) *fetcher {
	return &fetcher{
		cb:        make(map[string]callbackFunc),
		prefixLen: prefixLen,
	}
}

func (f *fetcher) Register(keyPrefix string, cbf callbackFunc) bool {
	if len(keyPrefix) != f.prefixLen {
		return false
	}
	f.cb[keyPrefix] = cbf
	return true
}

func (f *fetcher) Execute(key string) (interface{}, error) {
	if len(key) < f.prefixLen {
		return nil, fmt.Errorf(errInvalidKey, key)
	}
	keyPrefix := key[:f.prefixLen]
	cbf, ok := f.cb[keyPrefix]
	if !ok {
		return nil, fmt.Errorf(errInvalidKey, key)
	}
	return cbf(key)
}

/*

//header bidding
sample keys

Key_Prefix_PUB_SLOT_INFO  = "AAA00" -> DB
Key_Prefix_PUB_HB_PARTNER = "AAB00"
Key_Prefix_PubAdunitConfig = "AAC00"
Key_Prefix_PubSlotHashInfo = "AAD00"
Key_Prefix_PubSlotRegex    = "AAE00"
Key_Prefix_PubSlotNameHash = "AAF00"
Key_Prefix_PubVASTTags     = "AAG00" -> DBPubVasttags(key string) (interface,error)


PUB_SLOT_INFO  = Key_Prefix_PUB_SLOT_INFO + "_%d_%d_%d_%d" // publisher slot mapping at publisher, profile, display version and adapter level
PUB_HB_PARTNER = Key_Prefix_PUB_HB_PARTNER + "_%d_%d_%d"  // header bidding partner list at publishr,profile, display version level
PubAdunitConfig = Key_Prefix_PubAdunitConfig + "_%d_%d_%d"
PubSlotHashInfo = Key_Prefix_PubSlotHashInfo + "_%d_%d_%d_%d"     // slot and its hash info at publisher, profile, display version and adapter level
PubSlotRegex    = Key_Prefix_PubSlotRegex + "_%d_%d_%d_%d_%s" // slot and its matching regex info at publisher, profile, display version and adapter level
PubSlotNameHash = Key_Prefix_PubSlotNameHash + "_%d"       //publisher slotname hash mapping cache key
PubVASTTags     = Key_Prefix_PubVASTTags + "_%d"

func DBGetVASTTags(key string) (interface{},error) {
	strings.Split(key,"_")
	key[0] // keyprefix
	key[1] //publisherid
	return GetVastTag(rtb req, pub ID, comn etc)
}

//hb
fetcher.Register(Key_Prefix_PUB_SLOT_INFO, DBGetVASTTags)

type AsyncCache struct {
	c *cache
	f *fetcher
	ks *keystatus
}

func (ac *AsyncCache) aget(key string) {
	data, err := ac.f.execute(key)
	if err != nil {
		ac.c.set(key, data)
	}
}
*/
