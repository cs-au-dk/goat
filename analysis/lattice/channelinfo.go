package lattice

//go:generate go run generate-product.go channelinfo

// ChannelInfoLattice is the lattice of abstract channels.
type ChannelInfoLattice struct {
	ProductLattice
}

// ChannelInfo yields the abstract channel lattice.
func (latticeFactory) ChannelInfo() *ChannelInfoLattice {
	return channelInfoLattice
}

// channelInfoLattice is the singleton instantiation of the abstract channel lattice.
var channelInfoLattice *ChannelInfoLattice = &ChannelInfoLattice{
	*latFact.Product(
		// Capacity is an approximation of the channel capacity as a flat lattice.
		flatIntLattice,
		// Status encodes whether a channel is open or closed.
		latFact.Flat(true, false),
		// BufferFlat is an approximation of the current size of the channel's queue as a flat lattice.
		flatIntLattice,
		// BufferInterval approximates the size of the current queue as an interval lattice.
		intervalLattice,
		// Payload approximates the values stored in the channel's buffer.
		// Due to circular dependency with the abstract value lattice, the channel lattice
		// is extended with the payload component during initialization.
	),
}

func init() {
	// Extend channel lattice with the payload component, as the lattice of abstract values.
	channelInfoLattice.Extend(valueLattice)

	_checkChannelInfo(channelInfoLattice.Bot().ChannelInfo())
}

// Top returns the ⊤ value of an abstract channel.
func (c *ChannelInfoLattice) Top() Element {
	return ChannelInfo{
		element: element{lattice: c},
		product: c.ProductLattice.Top().Product(),
	}
}

// Bot returns the ⊥ value of an abstract channel.
func (c *ChannelInfoLattice) Bot() Element {
	return ChannelInfo{
		element: element{lattice: c},
		product: c.ProductLattice.Bot().Product(),
	}
}

// ChannelInfo creates an abstract channel value based on the provided values for
// capacity, status, and queue length approximations.
func (elementFactory) ChannelInfo(cap FlatElement, open bool, flat, inter Element) ChannelInfo {
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

// CapacityKnown checks whether the capacity is a known constant
func (m ChannelInfo) CapacityKnown() bool {
	return !(m.Capacity().IsTop() || m.Capacity().IsBot())
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
