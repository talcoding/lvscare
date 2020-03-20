package service

import (
	"fmt"
	"github.com/fanux/LVScare/internal/ipvs"
	"github.com/fanux/LVScare/utils"
	"net"
	"strconv"
)

//EndPoint  is
type EndPoint struct {
	IP   string
	Port uint16
}

func (ep EndPoint) String() string {
	port := strconv.Itoa(int(ep.Port))
	return ep.IP + ":" + port
}

//Lvser is
type Lvser interface {
	CreateVirtualServer(vs string) error
	GetVirtualServer() (vs *EndPoint, rs []*EndPoint)

	AddRealServer(rs string) error
	RemoveRealServer(rs string) error

	CheckRealServers(path, schem string)
}

type lvscare struct {
	vs     *EndPoint
	rs     []*EndPoint
	handle ipvs.Interface
}

func (l *lvscare) buildVirtualServer(vip string) *ipvs.VirtualServer {
	ip, port := utils.SplitServer(vip)
	virServer := &ipvs.VirtualServer{
		Address:   net.ParseIP(ip),
		Protocol:  "TCP",
		Port:      port,
		Scheduler: "rr",
		Flags:     0,
		Timeout:   0,
	}
	return virServer
}

func (l *lvscare) buildRealServer(real string) *ipvs.RealServer {
	ip, port := utils.SplitServer(real)
	realServer := &ipvs.RealServer{
		Address: net.ParseIP(ip),
		Port:    port,
		Weight:  1,
	}
	return realServer
}

func (l *lvscare) CreateVirtualServer(vs string) error {
	virIp, virPort := utils.SplitServer(vs)
	if virIp == "" || virPort == 0 {
		return fmt.Errorf("virtual server ip and port is null")
	}
	l.vs = &EndPoint{IP: virIp, Port: virPort}
	vServer := l.buildVirtualServer(vs)
	err := l.handle.AddVirtualServer(vServer)
	if err != nil {
		return fmt.Errorf("new virtual server failed: %s", err)
	}
	return nil
}

func (l *lvscare) AddRealServer(rs string) error {
	realIp, realPort := utils.SplitServer(rs)
	if realIp == "" || realPort == 0 {
		return fmt.Errorf("real server ip and port is null")
	}
	rsEp := &EndPoint{IP: realIp, Port: realPort}
	l.rs = append(l.rs, rsEp)
	realServer := l.buildRealServer(rs)
	//vir server build
	vServer := l.buildVirtualServer(l.vs.String())
	err := l.handle.AddRealServer(vServer, realServer)
	if err != nil {
		return fmt.Errorf("new real server failed: %s", err)
	}
	return nil
}
func (l *lvscare) GetVirtualServer() (vs *EndPoint, rs []*EndPoint) {
	virArray, err := l.handle.GetVirtualServers()
	if err != nil {
		fmt.Println("vir servers is nil", err)
		return nil, nil
	}
	resultVirServer := l.buildVirtualServer(l.vs.String())
	for _, vir := range virArray {
		fmt.Printf("check vir ip: %s, port %v\n", vir.Address.String(), vir.Port)
		if vir.String() == resultVirServer.String() {
			return l.vs, l.rs
		}
	}
	return
}

func (l *lvscare) healthCheck(ip, port, path, shem string) bool {
	return utils.IsHTTPAPIHealth(ip, port, path, shem)
}

func (l *lvscare) CheckRealServers(path, schem string) {
	//if realserver unavilable remove it, if recover add it back
	for _, realServer := range l.rs {
		ip := realServer.IP
		port := strconv.Itoa(int(realServer.Port))
		if !l.healthCheck(ip, port, path, schem) {
			err := l.RemoveRealServer(realServer.String())
			if err != nil {
				fmt.Printf("remove real server failed %s:%s", realServer.IP, realServer.Port)
			}
		} else {
			rs, weight := l.GetRealServer(realServer.String())
			if weight == 0 {
				err := l.RemoveRealServer(realServer.String())
				fmt.Println("remove weight = 0 real server")
				if err != nil {
					fmt.Println("	Error remove weight = 0 real server failed", realServer.IP, realServer.Port)
				}
			}
			if rs == nil || weight == 0 {
				//add it back
				ip := realServer.IP
				port := strconv.Itoa(int(realServer.Port))
				err := l.AddRealServer(ip + ":" + port)
				if err != nil {
					fmt.Printf("add real server failed %s:%s", realServer.IP, realServer.Port)
				}
			}
		}
	}
}

func (l *lvscare) GetRealServer(rsHost string) (rs *EndPoint, weight int) {
	ip, port := utils.SplitServer(rsHost)
	vs := l.buildVirtualServer(l.vs.String())
	dstArray, err := l.handle.GetRealServers(vs)
	if err != nil {
		fmt.Printf("get real server failed %s : %d\n", ip, port)
		return nil, 0
	}
	dip := net.ParseIP(ip)
	for _, dst := range dstArray {
		fmt.Printf("check realserver ip: %s, port %d\n", dst.Address.String(), dst.Port)
		if dst.Address.Equal(dip) && dst.Port == port {
			return &EndPoint{IP: ip, Port: port}, dst.Weight
		}
	}
	return nil, 0
}

//
func (l *lvscare) RemoveRealServer(rs string) error {
	realIp, realPort := utils.SplitServer(rs)
	if realIp == "" || realPort == 0 {
		return fmt.Errorf("real server ip and port is null")
	}

	if l.vs == nil {
		return fmt.Errorf("virtual service is nil.")
	}
	virServer := l.buildVirtualServer(l.vs.String())
	realServer := l.buildRealServer(rs)
	err := l.handle.DeleteRealServer(virServer, realServer)
	if err != nil {
		return fmt.Errorf("real server delete error: %v", err)
	}
	//clean delete data
	var resultRS []*EndPoint
	for _, r := range l.rs {
		if r.IP == realIp && r.Port == realPort {
			continue
		} else {
			resultRS = append(resultRS, &EndPoint{
				IP:   r.IP,
				Port: r.Port,
			})
		}
	}
	l.rs = resultRS
	return nil
}
