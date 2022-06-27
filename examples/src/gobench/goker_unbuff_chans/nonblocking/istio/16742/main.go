package main

type rwmutex struct {
	w chan bool
	r chan bool
}

var (
	adsClients      = map[string]*XdsConnection{}
	adsClientsMutex = func() (lock rwmutex) {
		lock = rwmutex{
			w: make(chan bool),
			r: make(chan bool),
		}

		go func() {
			rCount := 0

			// As long as all locks are free, both a reader
			// and a writer may acquire the lock
			for {
				select {
				// If a writer acquires the lock, hold it until released
				case <-lock.w:
					lock.w <- false
					// If a reader acquires the lock, step into read-mode.
				case <-lock.r:
					// Increment the reader count
					rCount++
					// As long as not all readers are released, stay in read-mode.
					for rCount > 0 {
						select {
						// One reader released the lock
						case lock.r <- false:
							rCount--
							// One reader acquired the lock
						case <-lock.r:
							rCount++
						}
					}
				}
			}
		}()

		return lock
	}()
)

type Collection []struct{}

func BuildSidecarVirtualHostsFromConfigAndRegistry(proxyLabels Collection) {}

type ConfigGenerator interface {
	BuildHTTPRoutes(node *Proxy)
}

type ConfigGeneratorImpl struct{}

func (configgen *ConfigGeneratorImpl) BuildHTTPRoutes(node *Proxy) {
	configgen.buildSidecarOutboundHTTPRouteConfig(node)
}

func (configgen *ConfigGeneratorImpl) buildSidecarOutboundHTTPRouteConfig(node *Proxy) {
	BuildSidecarVirtualHostsFromConfigAndRegistry(node.WorkloadLabels)
}

type Proxy struct {
	WorkloadLabels Collection
}

type XdsConnection struct {
	modelNode *Proxy
}

func newXdsConnection() *XdsConnection {
	return &XdsConnection{
		modelNode: &Proxy{},
	}
}

type DiscoveryServer struct {
	ConfigGenerator ConfigGenerator
}

func (s *DiscoveryServer) addCon(con *XdsConnection) {
	adsClientsMutex.w <- true
	defer func() { <-adsClientsMutex.w }()
	adsClients["1"] = con
}

func (s *DiscoveryServer) StreamAggregatedResources() {
	con := newXdsConnection()
	s.addCon(con)
	s.pushRoute(con)
}

func (s *DiscoveryServer) generateRawRoutes(con *XdsConnection) {
	s.ConfigGenerator.BuildHTTPRoutes(con.modelNode)
}

func (s *DiscoveryServer) pushRoute(con *XdsConnection) {
	s.generateRawRoutes(con)
}

func (s *DiscoveryServer) WorkloadUpdate() {
	adsClientsMutex.r <- true
	for _, connection := range adsClients {
		connection.modelNode.WorkloadLabels = nil
	}
	<-adsClientsMutex.r
}

type XDSUpdater interface {
	WorkloadUpdate()
}

type MemServiceDiscovery struct {
	EDSUpdater XDSUpdater
}

func (sd *MemServiceDiscovery) AddWorkload() {
	sd.EDSUpdater.WorkloadUpdate()
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

func main() {
	var wg = func() (wg waitgroup) {
		wg = waitgroup{
			pool: make(chan int),
			wait: make(chan bool),
		}

		go func() {
			count := 0

			for {
				select {
				// The WaitGroup may wait so long as the count is 0.
				case wg.wait <- true:
				// The first pooled goroutine will prompt the WaitGroup to wait
				// and disregard all sends on Wait until all pooled goroutines unblock.
				case x := <-wg.pool:
					count += x
					// TODO: Simulate counter dropping below 0 panics.
					for count > 0 {
						select {
						case x := <-wg.pool:
							count += x
						// Caller should receive on wg.Pool to decrement counter
						case wg.pool <- 0:
							count--
						}
					}
				}
			}
		}()

		return
	}()
	wg.pool <- 3
	go func() {
		defer func() { <-wg.pool }()
		registry := &MemServiceDiscovery{
			EDSUpdater: &DiscoveryServer{
				ConfigGenerator: &ConfigGeneratorImpl{},
			},
		}
		go func() {
			defer func() { <-wg.pool }()
			registry.EDSUpdater.(*DiscoveryServer).StreamAggregatedResources()
		}()
		go func() {
			defer func() { <-wg.pool }()
			registry.AddWorkload()
		}()
	}()
	<-wg.wait
}
