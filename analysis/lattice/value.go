package lattice

import (
	"fmt"

	loc "github.com/cs-au-dk/goat/analysis/location"
	i "github.com/cs-au-dk/goat/utils/indenter"

	"golang.org/x/tools/go/ssa"
)

// AbstractValue is the type of members of the abstract Go value lattice.
type AbstractValue struct {
	element
	value Element
	// typ codifies the type of the abstract value. If typ == 0, then the value
	// is untyped, which should only apply to the abstract ⊥ value.
	typ int
}

// AbstractValue generates an abstract value based on the given configuration.
func (elementFactory) AbstractValue(config AbstractValueConfig) AbstractValue {
	var value Element
	typ := _UNTYPED

	switch {
	case config.Basic != nil:
		value = elFact.Constant(config.Basic)
		typ = _BASIC_VALUE
	case config.Channel:
		chanLat := valueLattice.Get(_CHAN_VALUE).Lifted().Lattice
		value = chanLat.Bot().ChannelInfo()
		typ = _CHAN_VALUE
	case config.Wildcard:
		value = valueLattice.Get(_WILDCARD_VALUE).Top()
		typ = _WILDCARD_VALUE
	case config.PointsTo != nil:
		value = elFact.PointsTo(*config.PointsTo...)
		typ = _POINTER_VALUE
	case config.Struct != nil:
		structLat := valueLattice.Get(_STRUCT_VALUE)
		value = MakeInfiniteMap[any](structLat)(config.Struct)
		typ = _STRUCT_VALUE
	case config.Mutex:
		mutexLat := valueLattice.Get(_MUTEX_VALUE)
		// The mutex zero value is an unlocked mutex
		value = elFact.Flat(mutexLat)(false)
		typ = _MUTEX_VALUE
	case config.RWMutex:
		value = elFact.RWMutex()
		typ = _RWMUTEX_VALUE
	case config.Cond:
		value = elFact.Cond()
		typ = _COND_VALUE
	case config.WaitGroup:
		// The waitgroup zero value is a zeroed counter
		value = elFact.FlatInt(0)
		typ = _WAITGROUP_VALUE
	}

	if typ == _UNTYPED {
		panic(fmt.Errorf("Badly formatted abstract value configuration: %s", config))
	}

	return AbstractValue{
		element{valueLattice},
		value,
		typ,
	}
}

// AbstractStruct constructs an abstract struct value from the given key-value
// pairs.
func (elementFactory) AbstractStruct(fields map[any]Element) AbstractValue {
	return elFact.AbstractValue(AbstractValueConfig{
		Struct: fields,
	})
}

// AbstractArray constructs an abstract array value where the content is the
// given abstract value.
func (elementFactory) AbstractArray(val Element) AbstractValue {
	return elFact.AbstractStruct(map[any]Element{
		loc.AINDEX: val,
	})
}

// AbstractClosure constructs an abstract closure value, where the bindings encode
// the values of the free variables.
func (elementFactory) AbstractClosure(f ssa.Value, bindings map[any]Element) AbstractValue {
	bindings[-1] = Elements().AbstractBasic(f)
	return elFact.AbstractStruct(bindings)
}

// AbstractMap creates an abstract value denoting a map.
func (elementFactory) AbstractMap(k, v Element) AbstractValue {
	return elFact.AbstractStruct(map[any]Element{
		"keys":   k,
		"values": v,
	})
}

// AbstractStructV is a variadic version of the AbstractStruct constructor.
func (elementFactory) AbstractStructV(elements ...Element) AbstractValue {
	mp := make(map[any]Element)
	for i, el := range elements {
		mp[i] = el
	}

	return elFact.AbstractStruct(mp)
}

// AbstractPointer is creates an abstract value encoding a points-to set
// from the given set of locations.
func (elementFactory) AbstractPointer(pt []loc.Location) AbstractValue {
	return elFact.AbstractValue(AbstractValueConfig{
		PointsTo: &pt,
	})
}

// AbstractPointerV is a variadic variant of the AbstractPointer constructor.
func (elementFactory) AbstractPointerV(pt ...loc.Location) AbstractValue {
	return elFact.AbstractValue(AbstractValueConfig{
		PointsTo: &pt,
	})
}

// AbstractChannel creates an abstract channel value.
func (elementFactory) AbstractChannel() AbstractValue {
	return elFact.AbstractValue(AbstractValueConfig{
		Channel: true,
	})
}

// AbstractBasic creates an abstract constant.
func (elementFactory) AbstractBasic(x any) AbstractValue {
	return elFact.AbstractValue(AbstractValueConfig{
		Basic: x,
	})
}

// AbstractMutex creates an abstract mutex value.
func (elementFactory) AbstractMutex() AbstractValue {
	return elFact.AbstractValue(AbstractValueConfig{
		Mutex: true,
	})
}

// AbstractRWMutex creates an abstract read-write mutex value.
func (elementFactory) AbstractRWMutex() AbstractValue {
	return elFact.AbstractValue(AbstractValueConfig{
		RWMutex: true,
	})
}

// AbstractCond creates an abstract conditional variable value.
func (elementFactory) AbstractCond() AbstractValue {
	return elFact.AbstractValue(AbstractValueConfig{
		Cond: true,
	})
}

// AbstractWaitGroup creates an abstract waitgroup value.
func (elementFactory) AbstractWaitGroup() AbstractValue {
	return elFact.AbstractValue(AbstractValueConfig{
		WaitGroup: true,
	})
}

// AbstractWildcard creates an unknown pointer value.
func (elementFactory) AbstractWildcard() AbstractValue {
	return elFact.AbstractValue(AbstractValueConfig{
		Wildcard: true,
	})
}

// AbstractValue can safely be converted to an abstract value.
func (m AbstractValue) AbstractValue() AbstractValue {
	return m
}

// ChannelInfo converts an abstract value to an abstract channel,
// by accessing the underlying abstract channel value.
func (m AbstractValue) ChannelInfo() ChannelInfo {
	return m.ChanValue()
}

// PointsTo converts an abstract value to a points-to set by
// accessing the underlying points-to set of an abstract value.
func (m AbstractValue) PointsTo() PointsTo {
	return m.PointerValue()
}

// PointsTo converts an abstract value to a member of a flat lattice
// accessing the underlying constant value.
func (m AbstractValue) Flat() FlatElement {
	return m.BasicValue()
}

// RWMutex converts an abstract value to an abstract read-write mutex by
// accessing the underlying read-write mutex value.
func (m AbstractValue) RWMutex() RWMutex {
	return m.RWMutexValue()
}

// IsClosure checks whether an abstract value denotes an abstract closure.
func (m AbstractValue) IsClosure() bool {
	return m.IsKnownStruct() && !m.StructValue().ForAll(func(k any, _ Element) bool {
		return k != -1
	})
}

// Closure retrieves the SSA function captured by an abstract closure.
func (m AbstractValue) Closure() *ssa.Function {
	typeCheckValuesEqual(m.typ, _STRUCT_VALUE)
	if !m.IsKnownStruct() {
		panic("Attempted to get closure off non-known closure? " + m.String())
	}
	return m.StructValue().Get(-1).Flat().Value().(*ssa.Function)
}

// Cond converts an abstract value to an abstract conditional variable by
// accessing the underlying conditional variable value.
func (m AbstractValue) Cond() Cond {
	return m.CondValue()
}

func (m AbstractValue) String() string {
	var str string
	switch m.typ {
	case _UNTYPED:
		return "⊥"
	case _MUTEX_VALUE:
		mu := m.value.Flat()
		switch {
		case mu.IsBot() || mu.IsTop():
			str += mu.String()
		case mu.Is(true):
			str += colorize.Element("LOCKED")
		case mu.Is(false):
			str += colorize.Element("UNLOCKED")
		}

		return str
	case _RWMUTEX_VALUE:
		rwmu := m.value.RWMutex()
		rlocks := rwmu.RLocks()
		mu := rwmu.Status()
		str += "{ " + colorize.Field("Status") + ": "
		switch {
		case mu.IsBot() || mu.IsTop():
			str += mu.String()
		case mu.Is(true):
			str += colorize.Element("LOCKED")
		case mu.Is(false):
			str += colorize.Element("UNLOCKED")
		}

		str += ", " + colorize.Field("RLocks") + ": " + rlocks.String() + " }"

		return str
	case _WILDCARD_VALUE:
		return colorize.Element("(*)")
	}

	switch {
	case m.IsArray():
		el := m.StructValue().Get(loc.AINDEX)
		return i.Indenter().
			Start(colorize.Element("[")).
			NestThunked(func() string { return el.String() }).
			End(colorize.Element("]"))
	case m.IsMap():
		key, val := m.StructValue().Get("keys"), m.StructValue().Get("values")

		return colorize.Lattice("map[") +
			key.String() +
			colorize.Lattice("]{") +
			val.String() +
			colorize.Lattice("}")
	}

	str += m.value.String()

	return str
}

// Height returns the height in the lattice of an abstract value.
func (m AbstractValue) Height() int {
	return m.value.Height()
}

// Update returns the current abstract value updated with value of `x`.
// It will update different components of the abstract value depending
// on the implementation of `x`.
func (m AbstractValue) Update(x Element) AbstractValue {
	// If updating the bottom element, create a new abstract value
	// where the type is set according to the incoming element.
	if m.typ == _UNTYPED {
		var Underlying func(l Lattice) Lattice
		Underlying = func(l Lattice) Lattice {
			switch l := l.(type) {
			case *Lifted:
				return Underlying(l.Lattice)
			case *Dropped:
				return Underlying(l.Lattice)
			default:
				return l
			}
		}

		switch Underlying(x.Lattice()).(type) {
		case *ChannelInfoLattice:
			m.typ = _CHAN_VALUE
		case *ConstantPropagationLattice:
			m.typ = _BASIC_VALUE
		case *InfiniteMapLattice[any]:
			m.typ = _STRUCT_VALUE
		case *PointsToLattice:
			m.typ = _POINTER_VALUE
		case *MutexLattice:
			m.typ = _MUTEX_VALUE
		case *RWMutexLattice:
			m.typ = _RWMUTEX_VALUE
		case *CondLattice:
			m.typ = _COND_VALUE
		case *FlatIntLattice:
			m.typ = _WAITGROUP_VALUE
		default:
			panic(fmt.Errorf("Updated the abstract value ⊥ with unknown element %s %T", x, x))
		}
	}
	checkLatticeMatchThunked(
		x.Lattice(),
		valueLattice.Product().Get(m.typ),
		func() string { return fmt.Sprintf("%s ⇨ %s", m, x) })
	m.value = x
	return m
}

// IsBasic checks that an abstract value encodes only a member of the
// constant propagation lattice.
func (m AbstractValue) IsBasic() bool {
	return m.typ == _BASIC_VALUE
}

// UpdateChan updates the underlying abstract channel value.
func (m AbstractValue) UpdateChan(x Element) AbstractValue {
	typeCheckValuesEqual(m.typ, _CHAN_VALUE)
	m.value = x
	return m
}

// IsChan checks that the abstract value represents a channel.
func (m AbstractValue) IsChan() bool {
	return m.typ == _CHAN_VALUE
}

// IsStruct checks that the abstract value represents a struct.
func (m AbstractValue) IsStruct() bool {
	return m.typ == _STRUCT_VALUE
}

// IsMap checks that the abstract value represents a map.
func (m AbstractValue) IsMap() bool {
	if m.IsKnownStruct() {
		hasKeys := false
		isMap := m.StructValue().ForAll(func(key any, _ Element) bool {
			hasKeys = true
			return key == "keys" || key == "values"
		})
		return isMap && hasKeys
	}

	return false
}

// IsArray checks that the abstract value represents an array.
func (m AbstractValue) IsArray() bool {
	if m.IsKnownStruct() {
		hasKeys := false
		isArr := m.StructValue().ForAll(func(key any, _ Element) bool {
			hasKeys = true
			return key == loc.AINDEX
		})
		return isArr && hasKeys
	}

	return false
}

// IsKnownStruct checks that an abstract value represents a struct that
// is not ⊥ or ⊤.
func (m AbstractValue) IsKnownStruct() bool {
	return m.IsStruct() && !(m.IsBotStruct() || m.IsTopStruct())
}

// IsTopStruct checks whether the abstract value is a ⊤ struct.
func (m AbstractValue) IsTopStruct() bool {
	typeCheckValuesEqual(m.typ, _STRUCT_VALUE)
	_, isDropped := m.value.(*DroppedTop)
	return isDropped
}

// IsBotStruct checks whether the abstract value is a ⊥ struct.
func (m AbstractValue) IsBotStruct() bool {
	typeCheckValuesEqual(m.typ, _STRUCT_VALUE)
	_, isLifted := m.value.(*LiftedBot)
	return isLifted
}

// IsPointer checks whether the abstract value is a points-to set.
func (m AbstractValue) IsPointer() bool {
	return m.typ == _POINTER_VALUE
}

// IsMutex checks whether the abstract value is a mutex.
func (m AbstractValue) IsMutex() bool {
	return m.typ == _MUTEX_VALUE
}

// IsRWMutex checks whether the abstract value is a read-write mutex.
func (m AbstractValue) IsRWMutex() bool {
	return m.typ == _RWMUTEX_VALUE
}

// IsLocker checks whether the abstract value is a mutex or read-write mutex.
func (m AbstractValue) IsLocker() bool {
	return m.typ == _MUTEX_VALUE || m.typ == _RWMUTEX_VALUE
}

// IsCond checks whether the abstract value is a conditional variable.
func (m AbstractValue) IsCond() bool {
	return m.typ == _COND_VALUE
}

// AddPointers computes an abstract value with the given locations added to
// the underlying points-to set, if the abstract value is a points-to set,
// or conditional variable.
func (m AbstractValue) AddPointers(ls ...loc.Location) AbstractValue {
	switch {
	case m.IsPointer():
		pt := m.PointerValue()
		for _, l := range ls {
			pt = pt.Add(l)
		}
		m = m.UpdatePointer(pt)
	case m.IsCond() && m.Cond().IsLockerKnown():
		pt := m.Cond().KnownLockers()
		for _, l := range ls {
			pt = pt.Add(l)
		}
		m = m.UpdateCond(m.CondValue().UpdateLocker(pt))
	}

	return m
}

// MutexValue retrieves the underlying mutex value.
func (m AbstractValue) MutexValue() FlatElement {
	typeCheckValuesEqual(m.typ, _MUTEX_VALUE)
	return m.value.Flat()
}

// UpdateMutex computes an abstract mutex value updated with the given value.
func (m AbstractValue) UpdateMutex(x Element) AbstractValue {
	typeCheckValuesEqual(m.typ, _MUTEX_VALUE)
	m.value = x
	return m
}

// UpdateRWMutex computes an abstract read-write mutex value updated with the given value.
func (m AbstractValue) UpdateRWMutex(x Element) AbstractValue {
	typeCheckValuesEqual(m.typ, _RWMUTEX_VALUE)
	m.value = x
	return m
}

// UpdatePointer computes a points-to set updated with the given value.
func (m AbstractValue) UpdatePointer(x Element) AbstractValue {
	typeCheckValuesEqual(m.typ, _POINTER_VALUE)
	m.value = x
	return m
}

// UpdateCond computes an abstract conditional variable value updated with the given value.
func (m AbstractValue) UpdateCond(x Element) AbstractValue {
	typeCheckValuesEqual(m.typ, _COND_VALUE)
	m.value = x
	return m
}

// WaitGroupValue retrieves the underlying waitgroup value. Performs dynamic type checking.
func (m AbstractValue) WaitGroupValue() FlatElement {
	typeCheckValuesEqual(m.typ, _WAITGROUP_VALUE)
	return m.value.Flat()
}

// UpdateWaitGroup computes an abstract waitgroup value updated with the given value.
func (m AbstractValue) UpdateWaitGroup(x FlatElement) AbstractValue {
	typeCheckValuesEqual(m.typ, _WAITGROUP_VALUE)
	m.value = x
	return m
}

// BasicValue retrieves the underlying constant. Performs dynamic type checking.
func (m AbstractValue) BasicValue() FlatElement {
	typeCheckValuesEqual(m.typ, _BASIC_VALUE)
	return m.value.Flat()
}

// ChanValue retrieves the underlying abstract channel. Performs dynamic type checking.
func (m AbstractValue) ChanValue() ChannelInfo {
	typeCheckValuesEqual(m.typ, _CHAN_VALUE)
	return m.value.ChannelInfo()
}

// StructValue retrieves the underlying struct. Performs dynamic type checking.
func (m AbstractValue) StructValue() InfiniteMap[any] {
	typeCheckValuesEqual(m.typ, _STRUCT_VALUE)
	return m.value.(InfiniteMap[any])
}

// ArrayValue retrieves the underlying array. Performs dynamic type checking.
func (m AbstractValue) ArrayValue() InfiniteMap[any] {
	s := m.StructValue()
	if s.Contains(loc.AINDEX) && s.Size() == 1 {
		return s
	}
	panic(errUnsupportedTypeConversion)
}

// ArrayElementValue retrieves the contents of the underlying array. Performs dynamic type checking.
func (m AbstractValue) ArrayElementValue() AbstractValue {
	return m.ArrayValue().Get(loc.AINDEX).AbstractValue()
}

// RWMutexValue retrieves the read-write mutex value. Performs dynamic type checking.
func (m AbstractValue) RWMutexValue() RWMutex {
	typeCheckValuesEqual(m.typ, _RWMUTEX_VALUE)
	return m.value.RWMutex()
}

// CondValue retrieves the underlying conditional variable. Performs dynamic type checking.
func (m AbstractValue) CondValue() Cond {
	typeCheckValuesEqual(m.typ, _COND_VALUE)
	return m.value.Cond()
}

// Struct retrieves the underlying struct component without coercing into an infinite map
func (m AbstractValue) Struct() Element {
	typeCheckValuesEqual(m.typ, _STRUCT_VALUE)
	return m.value
}

// PointerValue retrieves the underlying non-⊤ points-to value.
func (m AbstractValue) PointerValue() PointsTo {
	if m.typ == _WILDCARD_VALUE {
		panic("Attempted to get PointerValue from Wildcard")
	}
	typeCheckValuesEqual(m.typ, _POINTER_VALUE)
	return m.value.PointsTo()
}

// Eq computes m = o. Performs lattice dynamic type checking.
func (e1 AbstractValue) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

// eq computes m = o.
func (e1 AbstractValue) eq(e2 Element) bool {
	switch e2 := e2.(type) {
	case AbstractValue:
		if e1 == e2 {
			return true
		}

		typeCheckValues(e1.typ, e2.typ)
		return e1.typ == e2.typ && e1.value.eq(e2.value)
	case *LiftedBot:
		return false
	case *DroppedTop:
		return false
	default:
		panic(errInternal)
	}
}

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (e1 AbstractValue) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

// geq computes m ⊒ o.
func (e1 AbstractValue) geq(e2 Element) bool {
	return e2.leq(e1) // OBS
}

// Leq computes m ⊑ o. Performs lattice dynamic type checking.
func (e1 AbstractValue) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

// leq computes m ⊑ o.
func (e1 AbstractValue) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case AbstractValue:
		switch {
		case e1.IsBot():
			return true
		case e2.IsBot():
			return false
		case e1.typ == _POINTER_VALUE && e2.typ == _WILDCARD_VALUE:
			return true
		case e1.typ == _WILDCARD_VALUE && e2.typ == _POINTER_VALUE:
			return false
		}
		typeCheckValuesEqual(e1.typ, e2.typ)
		return e1.value.leq(e2.value)
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

// Join computes m ⊔ o. Performs lattice dynamic type checking.
func (e1 AbstractValue) Join(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊔")
	return e1.join(e2)
}

// join computes m ⊔ o.
func (e1 AbstractValue) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case AbstractValue:
		return e1.MonoJoin(e2)
	case *LiftedBot:
		return e1
	case *DroppedTop:
		return e2
	default:
		panic(errInternal)
	}
}

// MonoJoin is a monomorphic variant of m ⊔ o for abstract values.
func (e1 AbstractValue) MonoJoin(e2 AbstractValue) AbstractValue {
	if e1 == e2 {
		return e1
	}

	switch {
	case e1.IsBot():
		return e2
	case e2.IsBot():
		return e1
	case e1.typ == _WILDCARD_VALUE && e2.typ == _POINTER_VALUE:
		return e1
	case e1.typ == _POINTER_VALUE && e2.typ == _WILDCARD_VALUE:
		return e2
	}

	typeCheckValuesEqual(e1.typ, e2.typ)
	e1.value = e1.value.join(e2.value)
	return e1
}

// Meet computes m ⊓ o. Performs lattice dynamic type checking.
func (e1 AbstractValue) Meet(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊓")
	return e1.meet(e2)
}

// meet computes m ⊓ o.
func (e1 AbstractValue) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case AbstractValue:
		return e1.MonoMeet(e2)
	case *LiftedBot:
		return e1
	case *DroppedTop:
		return e2
	default:
		panic(errInternal)
	}
}

// MonoMeet is a monomorphic variant of m ⊓ o for inter-processual analysis result.
func (e1 AbstractValue) MonoMeet(e2 AbstractValue) AbstractValue {
	if e1 == e2 {
		return e1
	}

	switch {
	case e1.IsBot():
		return e1
	case e2.IsBot():
		return e2
	case e1.typ == _WILDCARD_VALUE && e2.typ == _POINTER_VALUE:
		return e2
	case e1.typ == _POINTER_VALUE && e2.typ == _WILDCARD_VALUE:
		return e1
	}

	typeCheckValuesEqual(e1.typ, e2.typ)
	e1.value = e1.value.meet(e2.value)
	return e1
}

// IsBot checks whether the abstract value is the untyped ⊥ value.
func (e AbstractValue) IsBot() bool {
	return e.typ == _UNTYPED
}

// ToTop computes the corresponding an abstract value wrapping the ⊤ value
// for the given underlying value type.
func (e AbstractValue) ToTop() AbstractValue {
	switch e.typ {
	case _UNTYPED:
		return e
	case _POINTER_VALUE:
		return Consts().WildcardValue()
	case _CHAN_VALUE:
		// Setting a channel value to top requires some special handling
		status := Consts().Closed().Join(Consts().Open()).Flat()
		topbasic := Elements().FlatInt(0).Lattice().Top().Flat()
		topinter := Create().Lattice().Interval().Top().Interval()

		return e.Update(Elements().AbstractChannel().
			ChanValue().
			UpdateStatus(status).
			UpdateCapacity(topbasic).
			UpdateBufferFlat(topbasic).
			UpdateBufferInterval(topinter).
			UpdatePayload(e.ChanValue().Payload().ToTop()))
	case _STRUCT_VALUE:
		if e.IsClosure() {
			panic("Do not call ToTop() on a closure.")
		}
		// If the struct is a top map, skip it
		if e.IsTopStruct() {
			return e
		}
		if !e.IsBotStruct() {
			changed := false
			sv := e.StructValue()
			sv.ForEach(func(k any, v Element) {
				if topV := v.(AbstractValue).ToTop(); !topV.eq(v) {
					changed = true
					sv = sv.Update(k, topV)
				}
			})
			if changed {
				return e.Update(sv)
			}
			return e
		}
	}
	return e.Update(valueLattice.Get(e.typ).Top())
}

// IsWildcard checks whether the abstract value is the unknown pointer.
func (e AbstractValue) IsWildcard() bool {
	return e.typ == _WILDCARD_VALUE
}

// ToBot computes the corresponding an abstract value wrapping the ⊥ value
// for the given underlying value type.
func (e AbstractValue) ToBot() AbstractValue {
	switch {
	case e.typ == _STRUCT_VALUE:
		if e.IsClosure() {
			panic("Do not call ToBot() on a closure.")
		}
		// If the struct is a top map, skip it
		if e.IsBotStruct() {
			return e
		}

		if e.IsKnownStruct() {
			var size int
			e.ForEachField(func(i any, av AbstractValue) {
				size++
			})
			if size > 0 {
				changed := false
				sv := e.StructValue()
				sv.ForEach(func(k any, v Element) {
					if botV := v.(AbstractValue).ToBot(); !botV.eq(v) {
						changed = true
						sv = sv.Update(k, botV)
					}
				})
				if changed {
					return e.Update(sv)
				}
				return e
			}
		}
		fallthrough
	case e.typ != _UNTYPED:
		return e.Update(valueLattice.Get(e.typ).Bot())
	}
	return valueLattice.Bot().AbstractValue()
}

// ForEachField computes the given procedure for each field-value pair of a given
// struct value.
func (e AbstractValue) ForEachField(do func(any, AbstractValue)) {
	if e.typ == _STRUCT_VALUE {
		sv := e.StructValue()
		sv.ForEach(func(k any, v Element) {
			do(k, v.(AbstractValue))
		})
	}
}

// HasFixedHeight is true for constants, ⊤-structs, lockers, unknown pointers,
// and ⊥-structs.
func (v AbstractValue) HasFixedHeight() bool {
	return v.IsBasic() || v.IsTopStruct() ||
		v.IsLocker() || v.IsWildcard() || v.IsBotStruct()
}

// Difference recursively create a difference between values with variable-height lattices.
// It takes the receiver abstract value as the "base", and computes increases compared
// to the parameter abstract value.
func (v1 AbstractValue) Difference(v2 AbstractValue) (AbstractValue, bool) {
	// If either values is of a fixed-height lattice, the difference is
	// not relevant.
	switch {
	// case v1.HasFixedHeight() || v2.HasFixedHeight():
	// 	return Consts().BotValue(), false
	case v1.IsKnownStruct() && v2.IsKnownStruct():
		fields := make(map[any]Element)
		// sv
		v1.ForEachField(func(i any, av AbstractValue) {
			fv := v2.StructValue().Get(i).AbstractValue()
			if !fv.IsBot() {
				fvdiff, relevant := av.Difference(fv)
				if relevant {
					fields[i] = fvdiff
				}
			}
		})

		return Elements().AbstractStruct(fields), len(fields) > 0
	case v1.IsPointer() && v2.IsPointer():
		v := v2.PointerValue().Filter(func(l loc.Location) bool {
			al, ok := l.(loc.AddressableLocation)
			if !ok || IsTopLocation(l) {
				return !v1.PointerValue().Contains(l)
			}
			Tl, ok := representative(al)
			if !ok {
				return !v1.PointerValue().Contains(l)
			}

			if v1.PointerValue().Contains(Tl) {
				return false
			}
			return !v1.PointerValue().Contains(l)
		})
		return Elements().AbstractPointer(v.Entries()), !v.Empty()
	case v1.IsChan() && v2.IsChan():
		c1, c2 := v1.ChanValue(), v2.ChanValue()
		p1, p2 := c1.Payload(), c2.Payload()
		pvdiff, relevant := p1.Difference(p2)
		return Elements().AbstractChannel().
				Update(c2.UpdatePayload(pvdiff)),
			relevant
	case v1.IsCond() && v2.IsCond():
		c1, c2 := v1.CondValue(), v2.CondValue()
		if c1.IsLockerKnown() && c2.IsLockerKnown() {
			p1 := Elements().AbstractPointer(c1.KnownLockers().Entries())
			p2 := Elements().AbstractPointer(c2.KnownLockers().Entries())
			return p1.Difference(p2)
		}
	}
	return Consts().BotValue(), false
}

// InjectTopLocation inserts the ⊤-location into the value. Recursively traverse the
// abstract value and replace references to other locations represented by the
// given top location with the top location itself. Depending on value type:
//
// - 	Pointer values insert the represented location and remove all references
// to other locations represented by the top location
//
// - 	Struct values recursively inject fields
//
// -	Channel values recursively inject the payload
//
// -	All other values are returned as-is
func (e AbstractValue) InjectTopLocation(l loc.AddressableLocation) AbstractValue {
	switch e.typ {
	case _POINTER_VALUE:
		// Get underlying points-to set
		pt := e.PointerValue()
		// Replace all locations in the points-to set which are represented
		// by the top location with the top location
		pt = pt.Filter(func(pl loc.Location) bool {
			return !represents(l, pl)
		})
		// If any location was filtered from the points-to set,
		// then a top value could have been injected
		if pt.Size() < e.PointerValue().Size() {
			return e.UpdatePointer(pt.Add(l))
		}
		return e
	case _STRUCT_VALUE:
		// If the struct is a top map, skip it
		if e.IsTopStruct() {
			return e
		}
		// For struct values, recursively update each field
		sv := e.StructValue()
		e.ForEachField(func(i any, av AbstractValue) {
			sv = sv.Update(i, av.InjectTopLocation(l))
		})
		return e.Update(sv)
	case _CHAN_VALUE:
		// For channel values, recursively update the payload
		cv := e.ChanValue()
		pl := cv.Payload().InjectTopLocation(l)
		return e.Update(cv.UpdatePayload(pl))
	default:
		// No changes for other types of values
		return e
	}
}
