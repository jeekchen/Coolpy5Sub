package Nodes

import (
	"reflect"
	"github.com/garyburd/redigo/redis"
	"Coolpy/Incr"
	"encoding/json"
	"strconv"
	"Coolpy/Controller"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"strings"
	"Coolpy/Deller"
	"time"
)

type Node struct {
	Id    int64
	HubId int64 `validate:"required"`
	Title string `validate:"required"`
	About string
	Tags  []string
	Type  int `validate:"required"`
}

type NodeType struct {
	Switcher     int
	GenControl   int
	RangeControl int
	Value        int
	Gps          int
	Gen          int
	Photo        int
}

var NodeTypeEnum = NodeType{1, 2, 3, 4, 5, 6, 7}
var NodeReflectType = reflect.TypeOf(NodeTypeEnum)

func (c *NodeType) GetName(v int) string {
	return NodeReflectType.Field(v).Name
}

var rdsPool *redis.Pool

func Connect(addr string, pwd string) {
	rdsPool = &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: time.Second * 300,
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", addr)
			if err != nil {
				return nil, err
			}
			_, err = conn.Do("AUTH", pwd)
			if err != nil {
				return nil, err
			}
			conn.Do("SELECT", "3")
			return conn, nil
		},
	}
	go delChan()
}

func delChan() {
	for {
		select {
		case ukeyhid, ok := <-Deller.DelNodes:
			if ok {
				ns, err := nodeStartWith(ukeyhid + ":")
				if err != nil {
					break
				}
				for _, v := range ns {
					delnodes := ukeyhid + ":" + strconv.FormatInt(v.Id, 10)
					del(delnodes)
					go deldodps(delnodes)
				}
			}
		case delnode, ok := <-Deller.DelNode:
			if ok {
				del(delnode)
				go deldodps(delnode)
			}
		}
		if Deller.DelNodes == nil && Deller.DelNode == nil {
			break
		}
	}
}

func deldodps(ukeyhidnid string) {
	dpk := strings.Replace(ukeyhidnid, ":", ",", -1)
	go func(){
		Deller.DelValues <- dpk
	}()
	go func(){
		Deller.DelGpss <- dpk
	}()
	go func(){
		Deller.DelGens <- dpk
	}()
	go func(){
		Deller.DelPhotos <- dpk
	}()
}

func nodeCreate(ukey string, node *Node) error {
	v, err := Incr.NodeInrc()
	if err != nil {
		return err
	}
	node.Id = v
	json, err := json.Marshal(node)
	if err != nil {
		return err
	}
	key := ukey + ":" + strconv.FormatInt(node.HubId, 10) + ":" + strconv.FormatInt(node.Id, 10)
	rds := rdsPool.Get()
	defer rds.Close()
	_, err = rds.Do("SET", key, json)
	if err != nil {
		return err
	}
	//验证nodetype
	if NodeTypeEnum.GetName(node.Type - 1) == "" {
		return errors.New("node type error")
	}
	//初始化控制器
	if node.Type == NodeTypeEnum.Switcher {
		err := Controller.BeginSwitcher(ukey, node.HubId, node.Id)
		if err != nil {
			return errors.New("init error")
		}
	} else if node.Type == NodeTypeEnum.GenControl {
		err := Controller.BeginGenControl(ukey, node.HubId, node.Id)
		if err != nil {
			return errors.New("init error")
		}
	} else if node.Type == NodeTypeEnum.RangeControl {
		err := Controller.BeginRangeControl(ukey, node.HubId, node.Id)
		if err != nil {
			return errors.New("init error")
		}
	}
	return nil
}

func nodeStartWith(k string) ([]*Node, error) {
	rds := rdsPool.Get()
	defer rds.Close()
	data, err := redis.Strings(rds.Do("KEYSSTART", k))
	if err != nil {
		return nil, err
	}
	if len(data) <= 0 {
		return nil, errors.New("no data")
	}
	var ndata []*Node
	for _, v := range data {
		o, _ := redis.String(rds.Do("GET", v))
		h := &Node{}
		json.Unmarshal([]byte(o), &h)
		ndata = append(ndata, h)
	}
	return ndata, nil
}

func NodeGetOne(k string) (*Node, error) {
	rds := rdsPool.Get()
	defer rds.Close()
	o, err := redis.String(rds.Do("GET", k))
	if err != nil {
		return nil, err
	}
	h := &Node{}
	err = json.Unmarshal([]byte(o), &h)
	if err != nil{
		return nil,err
	}
	return h, nil
}

func nodeReplace(k string, h *Node) error {
	json, err := json.Marshal(h)
	if err != nil {
		return err
	}
	rds := rdsPool.Get()
	defer rds.Close()
	_, err = rds.Do("SET", k, json)
	if err != nil {
		return err
	}
	return nil
}

func del(k string) error {
	if len(strings.TrimSpace(k)) == 0 {
		return errors.New("uid was nil")
	}
	rds := rdsPool.Get()
	defer rds.Close()
	_, err := redis.Int(rds.Do("DEL", k))
	if err != nil {
		return err
	}
	return nil
}

func All() ([]string, error) {
	rds := rdsPool.Get()
	defer rds.Close()
	data, err := redis.Strings(rds.Do("KEYS", "*"))
	if err != nil {
		return nil, err
	}
	return data, nil
}