package metrics

// Captures below are recorded from a production router running BIRD 2.17.5
// and anonymised: addresses are replaced by RFC 5737/5738 documentation ranges and
// host names by example.net. They are the fixtures for the OSPF topology parsers.

const neighborsCapture = `BIRD 2.17.5 ready.
ospf1:
Router ID   	Pri	     State     	DTime	Interface  Router IP
203.0.113.2	  1	Full/BDR  	18.398	eth1       198.51.100.132
198.51.100.5	128	Full/PtP  	19.966	wg_home    192.0.2.25
203.0.113.3	  1	Full/PtP  	16.864	wg_egw_eu1 192.0.2.54
203.0.113.4	  1	Full/PtP  	18.696	wg_egw_eu2 192.0.2.30
198.51.100.6	128	Full/PtP  	18.349	wg_ck      192.0.2.10
`
const interfacesCapture = `BIRD 2.17.5 ready.
ospf1:
Interface eth1 (198.51.100.128/28)
	Type: broadcast
	Area: 0.0.0.0 (0)
	State: DR
	Priority: 1
	Cost: 10
	ECMP weight: 1
	Hello timer: 5
	Wait timer: 10
	Dead timer: 20
	Retransmit timer: 2
	Designated router (ID): 203.0.113.1
	Designated router (IP): 198.51.100.133
	Backup designated router (ID): 203.0.113.2
	Backup designated router (IP): 198.51.100.132
Interface wg_home (192.0.2.24/30)
	Type: ptp
	Area: 0.0.0.0 (0)
	State: PtP
	Priority: 1
	Cost: 10
	ECMP weight: 1
	Hello timer: 5
	Wait timer: 10
	Dead timer: 20
	Retransmit timer: 2
Interface wg_egw_eu1 (192.0.2.52/30)
	Type: ptp
	Area: 0.0.0.0 (0)
	State: PtP
	Priority: 1
	Cost: 10
	ECMP weight: 1
	Hello timer: 5
	Wait timer: 10
	Dead timer: 20
	Retransmit timer: 2
Interface wg_egw_eu2 (192.0.2.28/30)
	Type: ptp
	Area: 0.0.0.0 (0)
	State: PtP
	Priority: 1
	Cost: 10
	ECMP weight: 1
	Hello timer: 5
	Wait timer: 10
	Dead timer: 20
	Retransmit timer: 2
Interface wg_ck (192.0.2.8/30)
	Type: ptp
	Area: 0.0.0.0 (0)
	State: PtP
	Priority: 1
	Cost: 10
	ECMP weight: 1
	Hello timer: 5
	Wait timer: 10
	Dead timer: 20
	Retransmit timer: 2
Interface wg_server (192.0.2.192/28)
	Type: ptp
	Area: 0.0.0.0 (0)
	State: PtP
	Priority: 1
	Cost: 10
	ECMP weight: 1
	Hello timer: 5
	Wait timer: 10
	Dead timer: 20
	Retransmit timer: 2
Interface br_float (198.51.100.3/32)
	Type: broadcast
	Area: 0.0.0.0 (0)
	State: Waiting (stub)
	Priority: 1
	Cost: 10
	ECMP weight: 1
	Hello timer: 10
	Wait timer: 40
	Dead timer: 40
	Retransmit timer: 5
	Designated router (ID): 0.0.0.0
	Designated router (IP): 0.0.0.0
	Backup designated router (ID): 0.0.0.0
	Backup designated router (IP): 0.0.0.0
`
const lsadbCapture = `BIRD 2.17.5 ready.

Global

 Type   LS ID           Router          Sequence   Age  Checksum
 0005  192.0.2.12  203.0.113.3     8000004e   299    3cfe
 0005  192.0.2.20  203.0.113.3     8000004e   299    eb47
 0005  192.0.2.52  203.0.113.3     8000004e   299    aa68
 0005  198.51.100.1   203.0.113.3     8000004e   299    b190
 0005  192.0.2.35  203.0.113.2    8000004e   314    477b
 0005  192.0.2.36  203.0.113.2    8000004e   314    3d84
 0005  192.0.2.43  203.0.113.2    8000004e   314    f6c3
 0005  192.0.2.44  203.0.113.2    8000004e   314    eccc
 0005  192.0.2.51  203.0.113.2    8000004e   314    a60c
 0005  192.0.2.59  203.0.113.2    8000004e   314    5654
 0005  198.51.100.2   203.0.113.2    8000004e   314    9946
 0005  198.51.100.143 203.0.113.2    8000004e   314    b7a9
 0005  192.0.2.4   203.0.113.4    8000004e   296    af1a
 0005  192.0.2.19  203.0.113.4    8000004e   296    19a1
 0005  192.0.2.28  203.0.113.4    8000004e   296    bef2
 0005  198.51.100.4   203.0.113.4    8000004e   296    b60f
 0005  192.0.2.11  203.0.113.1   8000004e   303    b68d
 0005  192.0.2.27  203.0.113.1   8000004e   303    161e
 0005  192.0.2.28  203.0.113.1   8000004e   303    0c27
 0005  192.0.2.52  203.0.113.1   8000004e   303    1bff
 0005  192.0.2.207 203.0.113.1   8000004e   303    becc
 0005  198.51.100.3   203.0.113.1   8000004e   303    0e3a
 0005  198.51.100.143 203.0.113.1   8000004e   303    3694

Area 0.0.0.0

 Type   LS ID           Router          Sequence   Age  Checksum
 0001  198.51.100.5   198.51.100.5    800000a6   281    088c
 0001  198.51.100.6   198.51.100.6    8000002f   542    2afa
 0001  203.0.113.3    203.0.113.3     8000170c  1731    352f
 0001  203.0.113.2   203.0.113.2    8000a411   286    8265
 0001  203.0.113.4   203.0.113.4    80002569   832    e16a
 0001  203.0.113.1  203.0.113.1   800019e5   284    baa8
 0002  198.51.100.133 203.0.113.1   8000019b  1390    3118
`
const topologyCapture = `BIRD 2.17.5 ready.

area 0.0.0.0

	router 203.0.113.3
		distance 10
		router 203.0.113.1 metric 10
		router 198.51.100.5 metric 15
		router 198.51.100.6 metric 15

	router 203.0.113.2
		distance 10
		router 198.51.100.5 metric 10
		router 198.51.100.6 metric 10
		router 198.51.100.6 metric 15
		network 198.51.100.128/28 metric 10

	router 203.0.113.4
		distance 10
		router 198.51.100.5 metric 15
		router 198.51.100.6 metric 15
		router 203.0.113.1 metric 10

	router 203.0.113.1
		distance 0
		router 198.51.100.5 metric 10
		router 203.0.113.3 metric 10
		router 203.0.113.4 metric 10
		router 198.51.100.6 metric 10
		network 198.51.100.128/28 metric 10

	router 198.51.100.5
		distance 10
		router 203.0.113.3 metric 35
		router 203.0.113.2 metric 30
		router 203.0.113.4 metric 35
		router 203.0.113.1 metric 30

	router 198.51.100.6
		distance 10
		router 203.0.113.3 metric 35
		router 203.0.113.2 metric 40
		router 203.0.113.2 metric 30
		router 203.0.113.4 metric 35
		router 203.0.113.1 metric 30

	network 198.51.100.128/28
		dr 203.0.113.1
		distance 10
		router 203.0.113.1
		router 203.0.113.2
`
const routesCapture = `BIRD 2.17.5 ready.
Table master4:
198.51.100.4/32     unicast [ospf1 16:45:22.661] * I (150/10) [203.0.113.4]
	via 192.0.2.30 on wg_egw_eu2
	Type: OSPF univ
	OSPF.metric1: 10
	OSPF.router_id: 203.0.113.4
192.0.2.4/30     unicast [ospf1 16:45:22.661] * I (150/25) [203.0.113.4]
	via 192.0.2.30 on wg_egw_eu2
	Type: OSPF univ
	OSPF.metric1: 25
	OSPF.router_id: 203.0.113.4
192.0.2.32/30    unicast [ospf1 2026-11-07] * I (150/25) [203.0.113.2]
	via 198.51.100.132 on eth1
	Type: OSPF univ
	OSPF.metric1: 25
	OSPF.router_id: 203.0.113.2
198.51.100.128/28   unicast [ospf1 2026-10-31] I (150/10) [203.0.113.1]
	dev eth1
	Type: OSPF univ
	OSPF.metric1: 10
	OSPF.router_id: 203.0.113.1
192.0.2.20/30    unicast [ospf1 2026-11-07] * I (150/25) [203.0.113.3]
	via 192.0.2.54 on wg_egw_eu1
	Type: OSPF univ
	OSPF.metric1: 25
	OSPF.router_id: 203.0.113.3
192.0.2.48/30    unicast [ospf1 2026-10-31] * I (150/20) [203.0.113.2]
	via 198.51.100.132 on eth1
	Type: OSPF univ
	OSPF.metric1: 20
	OSPF.router_id: 203.0.113.2
192.0.2.8/30     unicast [ospf1 2026-10-31] I (150/10) [203.0.113.1]
	dev wg_ck
	Type: OSPF univ
	OSPF.metric1: 10
	OSPF.router_id: 203.0.113.1
198.51.100.3/32     unicast [ospf1 2026-11-07] I (150/0) [203.0.113.1]
	dev br_float
	Type: OSPF univ
	OSPF.metric1: 0
	OSPF.router_id: 203.0.113.1
198.51.100.6/32     unicast [ospf1 09:59:35.661] * I (150/11) [198.51.100.6]
	via 192.0.2.10 on wg_ck
	Type: OSPF univ
	OSPF.metric1: 11
	OSPF.router_id: 198.51.100.6
192.0.2.36/30    unicast [ospf1 2026-11-07] * I (150/25) [203.0.113.2]
	via 198.51.100.132 on eth1
	Type: OSPF univ
	OSPF.metric1: 25
	OSPF.router_id: 203.0.113.2
192.0.2.24/30    unicast [ospf1 2026-10-31] I (150/10) [203.0.113.1]
	dev wg_home
	Type: OSPF univ
	OSPF.metric1: 10
	OSPF.router_id: 203.0.113.1
198.51.111.0/24      unicast [ospf1 22:24:41.178] * I (150/11) [198.51.100.5]
	via 192.0.2.25 on wg_home
	Type: OSPF univ
	OSPF.metric1: 11
	OSPF.router_id: 198.51.100.5
198.51.112.0/24      unicast [ospf1 22:24:41.178] * I (150/11) [198.51.100.5]
	via 192.0.2.25 on wg_home
	Type: OSPF univ
	OSPF.metric1: 11
	OSPF.router_id: 198.51.100.5
198.51.113.0/24      unicast [ospf1 22:24:41.178] * I (150/11) [198.51.100.5]
	via 192.0.2.25 on wg_home
	Type: OSPF univ
	OSPF.metric1: 11
	OSPF.router_id: 198.51.100.5
192.0.2.52/30    unicast [ospf1 2026-10-31] I (150/10) [203.0.113.1]
	dev wg_egw_eu1
	Type: OSPF univ
	OSPF.metric1: 10
	OSPF.router_id: 203.0.113.1
192.0.2.12/30    unicast [ospf1 2026-11-07] * I (150/25) [203.0.113.3]
	via 192.0.2.54 on wg_egw_eu1
	Type: OSPF univ
	OSPF.metric1: 25
	OSPF.router_id: 203.0.113.3
192.0.2.40/30    unicast [ospf1 2026-11-06] * I (150/20) [203.0.113.2]
	via 198.51.100.132 on eth1
	Type: OSPF univ
	OSPF.metric1: 20
	OSPF.router_id: 203.0.113.2
198.51.100.2/32     unicast [ospf1 2026-11-07] * I (150/10) [203.0.113.2]
	via 198.51.100.132 on eth1
	Type: OSPF univ
	OSPF.metric1: 10
	OSPF.router_id: 203.0.113.2
198.51.100.5/32     unicast [ospf1 22:24:41.178] * I (150/11) [198.51.100.5]
	via 192.0.2.25 on wg_home
	Type: OSPF univ
	OSPF.metric1: 11
	OSPF.router_id: 198.51.100.5
198.51.110.0/24      unicast [ospf1 22:24:41.178] * I (150/11) [198.51.100.5]
	via 192.0.2.25 on wg_home
	Type: OSPF univ
	OSPF.metric1: 11
	OSPF.router_id: 198.51.100.5
192.0.2.28/30    unicast [ospf1 2026-10-31] I (150/10) [203.0.113.1]
	dev wg_egw_eu2
	Type: OSPF univ
	OSPF.metric1: 10
	OSPF.router_id: 203.0.113.1
192.0.2.56/30    unicast [ospf1 2026-10-31] * I (150/20) [203.0.113.2]
	via 198.51.100.132 on eth1
	Type: OSPF univ
	OSPF.metric1: 20
	OSPF.router_id: 203.0.113.2
198.51.121.0/24      unicast [ospf1 09:59:18.661] * I (150/11) [198.51.100.6]
	via 192.0.2.10 on wg_ck
	Type: OSPF univ
	OSPF.metric1: 11
	OSPF.router_id: 198.51.100.6
192.0.2.16/30    unicast [ospf1 16:45:22.661] * I (150/25) [203.0.113.4]
	via 192.0.2.30 on wg_egw_eu2
	Type: OSPF univ
	OSPF.metric1: 25
	OSPF.router_id: 203.0.113.4
198.51.122.0/24      unicast [ospf1 09:59:18.661] * I (150/11) [198.51.100.6]
	via 192.0.2.10 on wg_ck
	Type: OSPF univ
	OSPF.metric1: 11
	OSPF.router_id: 198.51.100.6
198.51.123.0/24      unicast [ospf1 09:59:18.661] * I (150/11) [198.51.100.6]
	via 192.0.2.10 on wg_ck
	Type: OSPF univ
	OSPF.metric1: 11
	OSPF.router_id: 198.51.100.6
198.51.124.0/24      unicast [ospf1 09:59:18.661] * I (150/11) [198.51.100.6]
	via 192.0.2.10 on wg_ck
	Type: OSPF univ
	OSPF.metric1: 11
	OSPF.router_id: 198.51.100.6
198.51.100.1/32     unicast [ospf1 2026-11-07] * I (150/10) [203.0.113.3]
	via 192.0.2.54 on wg_egw_eu1
	Type: OSPF univ
	OSPF.metric1: 10
	OSPF.router_id: 203.0.113.3
192.0.2.44/30    unicast [ospf1 2026-11-06] * I (150/20) [203.0.113.2]
	via 198.51.100.132 on eth1
	Type: OSPF univ
	OSPF.metric1: 20
	OSPF.router_id: 203.0.113.2
192.0.2.192/28   unicast [ospf1 2026-10-31] I (150/10) [203.0.113.1]
	dev wg_server
	Type: OSPF univ
	OSPF.metric1: 10
	OSPF.router_id: 203.0.113.1
`
