package lattice

//go:generate go run generate-product.go ChannelInfo Capacity,FlatElement,Flat,Capacity Status,FlatElement,Flat,Open? "BufferFlat,FlatElement,Flat,Buffer (flat)" "BufferInterval,Interval,Interval,Buffer (interval)" Payload,AbstractValue,AbstractValue,Payload

/* Information about channels used during abstract interpretation. */
type ChannelInfoLattice struct {
	ProductLattice
}

func (latticeFactory) ChannelInfo() *ChannelInfoLattice {
	return channelInfoLattice
}

var channelInfoLattice *ChannelInfoLattice = &ChannelInfoLattice{
	*latFact.Product(
		// Capacity - a flat approximation of the channel capacity
		flatIntLattice,
		// Status - is a channel open or closed?
		latFact.Flat(true, false),
		// BufferFlat - flat information for channel buffers
		flatIntLattice,
		// BufferInterval - interval information for channel buffers
		intervalLattice,
		// Payload - information registered at the buffer
	),
}

func (c *ChannelInfoLattice) Top() Element {
	return ChannelInfo{
		element: element{lattice: c},
		product: c.ProductLattice.Top().Product(),
	}
}

func (c *ChannelInfoLattice) Bot() Element {
	return ChannelInfo{
		element: element{lattice: c},
		product: c.ProductLattice.Bot().Product(),
	}
}

func init() {
	// What a hack... mutual dependency between channel info and value lattice.
	// Extend with value for payload.
	channelInfoLattice.Extend(valueLattice)

	_checkChannelInfo(channelInfoLattice.Bot().ChannelInfo())
}

func (elementFactory) ChannelInfo(cap FlatElement, open bool, flat Element, inter Element) ChannelInfo {
	status := channelInfoLattice.Status()

	return ChannelInfo{
		element: element{channelInfoLattice},
		product: elFact.Product(channelInfoLattice.Product())(
			cap,
			elFact.Flat(status)(open),
			flat,
			inter,
		),
	}
}

func (l1 *ChannelInfoLattice) Eq(l2 Lattice) bool {
	switch l2 := l2.(type) {
	case *ChannelInfoLattice:
		return true
	case *Lifted:
		return l1.Eq(l2.Lattice)
	case *Dropped:
		return l1.Eq(l2.Lattice)
	default:
		return false
	}
}

func (*ChannelInfoLattice) String() string {
	return colorize.Lattice("Channel")
}

func (c *ChannelInfoLattice) Product() *ProductLattice {
	return c.ProductLattice.Product()
}

func (c *ChannelInfoLattice) ChannelInfo() *ChannelInfoLattice {
	return c
}

func (m ChannelInfo) JoinPayload(p AbstractValue) ChannelInfo {
	return m.UpdatePayload(m.Payload().MonoJoin(p.AbstractValue()))
}

func (m ChannelInfo) MeetStatus(c FlatElement) ChannelInfo {
	return m.UpdateStatus(m.Status().Meet(c).Flat())
}

// Checks whether the flat buffer is known
func (m ChannelInfo) BufferFlatKnown() bool {
	return !(m.BufferFlat().IsTop() || m.BufferFlat().IsBot())
}

// Checks whether the buffer encoding of the interval is known.
// Returns false if the interval is completely unkown, or the bottom interval value.
func (m ChannelInfo) BufferIntervalKnown() bool {
	switch i := m.product.Get(_BUFFER_INTERVAL).(type) {
	case Interval:
		return !i.IsBot() && !i.IsTop()
	default:
		panic(errInternal)
	}
}

const (
	_CAPACITY = iota
	_STATUS
	_BUFFER_FLAT
	_BUFFER_INTERVAL
	_PAYLOAD
)

func (c *ChannelInfoLattice) Capacity() *FlatIntLattice {
	return c.product[_CAPACITY].FlatInt()
}

func (c *ChannelInfoLattice) Status() *FlatFiniteLattice {
	return c.product[_STATUS].FlatFinite()
}

func (c *ChannelInfoLattice) BufferFlat() *FlatIntLattice {
	return c.product[_BUFFER_FLAT].FlatInt()
}

func (c *ChannelInfoLattice) BufferInterval() *IntervalLattice {
	return c.product[_BUFFER_INTERVAL].Interval()
}

func (c *ChannelInfoLattice) Payload() *AbstractValueLattice {
	return c.product[_PAYLOAD].AbstractValue()
}

// Checks whether the buffer is guaranteed to not be empty.
func (m ChannelInfo) BufferNonEmpty() (res bool) {
	res = !(m.BufferFlat().Is(0) || m.BufferFlat().IsBot())

	if m.BufferIntervalKnown() {
		low := m.BufferInterval().Low()
		res = res || low > 0
	}
	return res
}

// Checks whether the channel is guaranteed to be synchronous
func (m ChannelInfo) Synchronous() bool {
	return m.Capacity().Is(0)
}

// Checks whether the channel may be synchronous
func (m ChannelInfo) MaySynchronous() bool {
	return m.Capacity().Is(0) || m.Capacity().IsTop()
}

// Checks whether the channel is guaranteed to be asynchronous
func (m ChannelInfo) Asynchronous() bool {
	cap := m.Capacity()
	return !(cap.Is(0) || cap.IsBot() || cap.IsTop())
}

// Checkes whether the channel may be asynchronous
func (m ChannelInfo) MayAsynchronous() bool {
	return !(m.Capacity().IsBot() || m.Capacity().Is(0))
}
