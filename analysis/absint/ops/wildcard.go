package ops

import (
	"fmt"
	T "go/types"
	"log"

	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	u "github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/utils"

	loc "github.com/cs-au-dk/goat/analysis/location"

	"github.com/fatih/color"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

// labelsToLocs computes a set of locations from labels produced by the points-to analysis.
// The strategy for constructing locations is given by "mkLoc".
func labelsToLocs(pt pointer.Pointer, mkLoc func(*pointer.Label) loc.Location) []loc.Location {
	locMap := make(map[loc.Location]struct{})
	for _, l := range pt.PointsTo().Labels() {
		if l.Value() != nil {
			locMap[mkLoc(l)] = struct{}{}
		}
	}

	locs := make([]loc.Location, 0, len(locMap))

	for l := range locMap {
		locs = append(locs, l)
	}

	return locs
}

// labelsToAlloc computes a set of allocation sites, or allocation site access paths
// from labels produced by the points-to analysis.
func labelsToAllocs(pt pointer.Pointer) []loc.Location {
	return labelsToLocs(pt, func(l *pointer.Label) loc.Location {
		// Get root value and access path.
		v, accesses := u.SplitLabel(l)
		var ptr loc.Location
		if global, ok := v.(*ssa.Global); ok {
			// If the root value is a global variable, set the root allocation site
			// as a global location.
			ptr = loc.GlobalLocation{Site: global}
		} else {
			// If the root value is a local allocation site, create it
			// and assign it as belonging to the unknown goroutine.
			ptr = loc.AllocationSiteLocation{
				Goro:    defs.Create().TopGoro(),
				Site:    v,
				Context: v.Parent(),
			}
		}

		if len(accesses) == 0 {
			// If no access path is present, return the constructed allocation site directly.
			return ptr
		}

		// Keep track of the type as the location is constructed. This is required for infering the type of
		//
		var typ T.Type
		switch bTyp := v.Type().Underlying().(type) {
		case *T.Slice:
			// Slice allocation sites do not have *T.Pointer type.
			if _, ok := accesses[0].(u.ArrayAccess); !ok {
				log.Fatalf("Access path encodes non-array access action %v for slice value %v", accesses, v)
			}

			// Use up access path.
			accesses = accesses[1:]
			// Get type of underlying element
			typ = bTyp.Elem()
			// Build heap location as an array location.
			ptr = loc.NewArrayElementLocation(ptr)

		case *T.Pointer:
			typ = bTyp.Elem()

		default:
			log.Fatalf("Allocation site has unexpected type %T %v", bTyp, bTyp)
		}

		// For each remaining access path, incrementally construct the location.
		for _, access := range accesses {
			switch access := access.(type) {
			case u.FieldAccess:
				fieldName := access.Field

				structT := typ.Underlying().(*T.Struct)
				found := false
				for i := 0; i < structT.NumFields(); i++ {
					if field := structT.Field(i); field.Name() == fieldName {

						typ = field.Type()
						ptr = loc.FieldLocation{
							Base:  ptr,
							Index: i,
						}

						found = true
						break
					}
				}

				if !found {
					fmt.Println(v.Type(), l.Path())
					log.Fatalf("None of %v's fields has name: %s", structT, fieldName)
				}
			case u.ArrayAccess:
				typ = typ.Underlying().(*T.Array).Elem()
				ptr = loc.NewArrayElementLocation(ptr)
			}
		}

		return ptr
	})
}

// allocTopValue creates a top value for the given type.
func allocTopValue(t T.Type) L.AbstractValue {
	// Check whether to create top values for standard library primitive types.
	switch {
	case utils.IsNamedType(t, "sync", "Mutex"):
		return L.Elements().AbstractMutex().ToTop()
	case utils.IsNamedType(t, "sync", "RWMutex"):
		return L.Elements().AbstractRWMutex().ToTop()
	case utils.IsNamedType(t, "sync", "Cond"):
		return L.Elements().AbstractCond().ToTop()
	case utils.IsNamedType(t, "sync", "WaitGroup"):
		return L.Elements().AbstractWaitGroup().ToTop()
	}

	switch t := t.Underlying().(type) {
	case *T.Chan:
		// When the wildcard corresponds to a channel,
		// then the top channel value must be inserted for every
		// allocation site in the may-alias set.
		status := L.Consts().Closed().Join(L.Consts().Open()).Flat()
		topbasic := L.Elements().FlatInt(0).Lattice().Top().Flat()
		topinter := L.Create().Lattice().Interval().Top().Interval()

		ch := L.Elements().AbstractChannel().
			ChanValue().
			UpdateStatus(status).
			UpdateCapacity(topbasic).
			UpdateBufferFlat(topbasic).
			UpdateBufferInterval(topinter).
			UpdatePayload(L.TopValueForType(t.Elem()))

		return L.Elements().AbstractChannel().UpdateChan(ch)
	case *T.Map:
		// When the type is a map, construct a struct where
		// the "keys" and "values" fields are set to the top elements
		// for the given type
		return L.Elements().AbstractMap(
			L.TopValueForType(t.Key()),
			L.TopValueForType(t.Elem()))
	case *T.Slice:
		return L.Elements().AbstractArray(L.TopValueForType(t.Elem()))
	default:
		panic(fmt.Sprintf("Don't know how to construct top value for allocation site of type %s", t))
	}
}

// GetAllocationSiteLocation returns the allocation site of an arbitrary location value.
// For global and allocation sites, it acts as the identity function. Field locations
// are recursively traversed until the base is an addressable location. Any other location
// types will lead to panic.
func GetAllocationSiteLocation(l loc.Location) loc.AddressableLocation {
	switch l := l.(type) {
	case loc.GlobalLocation:
		return l
	case loc.AllocationSiteLocation:
		return l
	case loc.FieldLocation:
		return GetAllocationSiteLocation(l.Base)
	default:
		panic(fmt.Errorf("Cannot retrieve allocation site location from %v (%T)", l, l))
	}
}

// swapBase replaces the base for an arbitrary location with another allocation site.
// For non-field locations, it returns the new allocation site. For field locations,
// it recursively reconstructs the field location structure with the new allocation site
// as the base.
func swapBase(l loc.Location, newBase loc.AllocationSiteLocation) loc.Location {
	switch l := l.(type) {
	case loc.AllocationSiteLocation:
		return newBase
	case loc.FieldLocation:
		l.Base = swapBase(l.Base, newBase)
		return l
	default:
		panic(fmt.Errorf("Cannot swap base on %v (%T)", l, l))
	}
}

// swapCache memoizes points-to set trees for SSA values so they can quickly be checked for
// equality during the analysis. This avoids creating different but equivalent trees
// every time the same SSA value was wildcard-swapped.
//
// Yields a 30-40% speed-up according to (small) experiments.
var swapCache struct {
	pt    *u.PointerResult
	cache map[pointer.Pointer]L.PointsTo
}

// UnwrapWildcard returns the points-to set for the location pointed to by `l` based on the
// result of the upfront analysis. The returned memory contains ⊤-valued bindings for the
// necessary locations such that the returned pointers do not go "out-of-bounds".
func UnwrapWildcard(
	pt *u.PointerResult,
	mem L.Memory,
	l loc.Location,
	knownFocusedPrimitives map[ssa.Value]L.PointsTo,
) (L.AbstractValue, L.Memory) {
	// swapCache must be invalidated and reset whenever the points-to context changes.
	if pt != swapCache.pt {
		swapCache.pt = pt
		swapCache.cache = map[pointer.Pointer]L.PointsTo{}
	}

	fset := pt.CallGraph.Root.Func.Prog.Fset
	ssaSite, _ := l.GetSite()
	lTyp := l.Type()

	var preanalysisPointer pointer.Pointer
	switch l := l.(type) {
	case loc.FieldLocation:
		// Allow swapping of Locker field on Cond objects via CondQueries.
		if bt, ok := l.Base.Type().(*T.Pointer); ok &&
			utils.IsNamedTypeStrict(bt.Elem(), "sync", "Cond") &&
			bt.Elem().Underlying().(*T.Struct).Field(l.Index).Name() == "L" {
			var hasSite bool
			ssaSite, hasSite = l.Base.GetSite()
			if !hasSite {
				log.Panicf("Cond Locker field pointer to embedded Cond was not found? %v", l)
			}
			preanalysisPointer = *pt.CondQueries[ssaSite]
		}
	case loc.AddressableLocation:
		preanalysisPointer = pt.Queries[ssaSite]
	default:
		panic(fmt.Errorf("Unsupported wilcard swap of %v", l))
	}

	ptl, found := swapCache.cache[preanalysisPointer]
	if !found {
		// Get all aliases of the given pointer as top allocation sites
		locs := labelsToAllocs(preanalysisPointer)
		// Construct a points-to set including the nil location and all the top
		// allocation sites.
		ol := l
		ptl = L.Create().Element().PointsTo(locs...).Filter(func(l loc.Location) bool {
			// Get the site of the location
			site, ok := GetAllocationSiteLocation(l).GetSite()
			if !ok {
				log.Fatalln("No site for", l)
			}
			// Exclude slice locations that may have been created via calls to
			// "append" to alleviate points-to set size explosions.
			if site, ok := site.(*ssa.Call); ok {
				common := site.Call
				if f, ok := common.Value.(*ssa.Builtin); ok && f.Name() == "append" {
					return false
				}
			}

			// Filter out locations that don't match the type
			// NOTE: This would suggest that the pointer analysis is buggy?
			// 	Maybe it shouldn't fail silently?
			typesValid := utils.TypeCompat(lTyp, l.Type())
			if utils.Opts().Verbose() && ok && !typesValid {
				log.Println("Source site:", color.GreenString(site.Name()+" = "+site.String()))
				log.Println("Source site type: " +
					color.GreenString("%s ", l.Type()) +
					color.CyanString("%p", l.Type()))
				log.Println("Source site underlying type: " +
					color.GreenString("%s ", l.Type().Underlying()) +
					color.CyanString("%p", l.Type().Underlying()))
				if site.Parent() != nil {
					if site.Parent().Pkg != nil {
						if site.Parent().Pkg.Pkg != nil {
							log.Println("Package: " + color.GreenString("%s", site.Parent().Pkg.Pkg) +
								color.CyanString("%p", site.Parent().Pkg.Pkg))
						} else {
							log.Println("Package of package is nil: " + color.GreenString("%s ", site.Parent().Pkg) +
								color.CyanString("%p", site.Parent().Pkg))
						}
					} else {
						log.Println("Package of parent is nil: " + color.GreenString("%s ", site.Parent()) +
							color.CyanString("%p", site.Parent()))
					}
				} else {
					log.Println("Parent is nil: " + color.GreenString("%s ", site.Parent()) +
						color.CyanString("%p", site.Parent()))
				}
				log.Println("Target site:", color.RedString(ol.String()))
				log.Println("Target site type: " +
					color.RedString("%s ", lTyp) +
					color.CyanString("%p", lTyp))
				log.Println("Target site underlying type: " +
					color.RedString("%s ", lTyp.Underlying()) +
					color.CyanString("%p", lTyp.Underlying()))
				/*
					if ssaVal.Parent() != nil {
						if ssaVal.Parent().Pkg != nil {
							if ssaVal.Parent().Pkg.Pkg != nil {
								log.Println("Package: " + color.RedString("%s ", ssaVal.Parent().Pkg.Pkg) +
									color.CyanString("%p", ssaVal.Parent().Pkg.Pkg))
							} else {
								log.Println("Package of package is nil: " + color.RedString("%s ", ssaVal.Parent().Pkg) +
									color.CyanString("%p", ssaVal.Parent().Pkg))
							}
						} else {
							log.Println("Package of parent is nil: " + color.RedString("%s ", ssaVal.Parent()) +
								color.CyanString("%p", ssaVal.Parent()))
						}
					} else {
						log.Println("Parent is nil: " + color.RedString("%s ", ssaVal.Parent()) +
							color.CyanString("%p", ssaVal.Parent()))
					}
				*/
				fmt.Println()
			}
			return typesValid
		}).Add(loc.NilLocation{}) // Nil must be included for soundness
		swapCache.cache[preanalysisPointer] = ptl
	}

	if len(knownFocusedPrimitives) > 0 {
		// For each known focused primitive, replace the ⊤-owned allocation site
		// pointer with the set of known possible allocations.
		ptl.ForEach(func(l loc.Location) {
			if _, isNil := l.(loc.NilLocation); isNil {
				return
			}

			if aloc, ok := GetAllocationSiteLocation(l).(loc.AllocationSiteLocation); ok {
				if ptsto, found := knownFocusedPrimitives[aloc.Site]; found {
					swapped := make([]loc.Location, 0, ptsto.Size())
					ptsto.ForEach(func(nl loc.Location) {
						swapped = append(swapped, swapBase(l, nl.(loc.AllocationSiteLocation)))
					})
					ptl = ptl.Remove(l).MonoJoin(L.Elements().PointsTo(swapped...))
				}
			}
		})
	}

	allocateOrSet := func(key loc.AddressableLocation, value L.AbstractValue) {
		if l, isAllocSite := key.(loc.AllocationSiteLocation); isAllocSite {
			mem = mem.Allocate(l, value, true)
		} else {
			mem = mem.Update(key, value)
		}
	}

	// Only perform case analysis on locations which are not nil
	ptl.FilterNil().ForEach(func(l2_ loc.Location) {
		// l2 might be a FieldLocation, so we might need to allocate a value for the base.
		l2 := GetAllocationSiteLocation(l2_)
		site, _ := l2.GetSite()

		// Skip pessimistic over-approximation for known primitives
		if aloc, ok := l2.(loc.AllocationSiteLocation); ok &&
			!defs.Create().TopGoro().Equal(aloc.Goro.(defs.Goro)) {
			return
		}

		switch t := site.Type().Underlying().(type) {
		case *T.Pointer:
			// Update every member of the representative points-to set
			// to bind to a top value.
			defer func() {
				if err := recover(); err != nil {
					mops := L.MemOps(mem)
					v, _ := mops.Get(l)
					v2, _ := mops.Get(l2)
					fmt.Println("Original location", l, "has site", ssaSite, "of type", lTyp)
					fmt.Println("Points to sites: {")
					locs := labelsToAllocs(preanalysisPointer)
					for _, l := range locs {
						siteVal, _ := GetAllocationSiteLocation(l).GetSite()
						fmt.Println(l, "of type", siteVal.Type())
						fmt.Println("Original construct at: ", fset.Position(siteVal.Pos()))
					}
					fmt.Println("Indirectly points-to {")
					for _, l := range pt.IndirectQueries[ssaSite].PointsTo().Labels() {
						fmt.Println(l.Value(), "of type", l.Value().Type())
						fmt.Println("Original construct at: ", fset.Position(l.Value().Pos()))
					}
					fmt.Println("}")
					fmt.Println("Type of element under pointer: ", t.Elem().Underlying())
					fmt.Println("Top value of underlying element type: ", L.TopValueForType(t.Elem()))
					fmt.Println("Location in points-to set", l2)
					fmt.Println("Initial value for swapped location:", v)
					fmt.Println("Initial value for location l2:", v2)
					panic(err)
				}
			}()

			allocateOrSet(l2, L.TopValueForType(t.Elem()))

		case *T.Interface:
			// For interface, examine the SSA interface allocation site (make interface{} <- x)
			// for the type of x to construct a top value.
			s, _ := l2.GetSite()
			if sItf, ok := s.(*ssa.MakeInterface); ok {
				// For Locker interfaces, ensure that all possible underlying standard
				// API top mutexes are instantiated and updated in memory.
				if utils.IsNamedType(sItf.X.Type(), "sync", "Mutex") ||
					utils.IsNamedType(sItf.X.Type(), "sync", "RWMutex") {
					// The type system ensures that Locker
					// interfaces can only be made from *sync.Mutex or *sync.RWmutex
					// (they always use a pointer receiver)
					locs := labelsToAllocs(pt.Queries[sItf.X])
					for _, l_ := range locs {
						l := GetAllocationSiteLocation(l_)
						site, _ := l.GetSite()
						allocT := site.Type().Underlying().(*T.Pointer).Elem()
						allocateOrSet(l, L.TopValueForType(allocT))
					}

					// For soundness, the list of pointers must include nil
					ptl := L.Elements().AbstractPointer(append(locs, loc.NilLocation{}))
					allocateOrSet(l2, ptl)
				} else {
					// Make an allocation site
					allocateOrSet(l2, L.TopValueForType(sItf.X.Type()))
				}
			} else {
				panic(fmt.Sprintf("Allocation site of interface %s is not a MakeInterface instruction?", s))
			}

		case *T.Signature:
			// Reconstruct the location set as of functions
			s, _ := l2.GetSite()

			if f, ok := s.(*ssa.Function); ok {
				// The closure is an abstract structure that
				// must create bindings for all free variables
				bindings := make(map[interface{}]L.Element)

				for i, fv := range f.FreeVars {
					bindings[i] = L.TopValueForType(fv.Type())
				}

				allocateOrSet(l2, L.Elements().AbstractClosure(f, bindings))
			} else {
				panic(fmt.Sprintf("Allocation site of function %s is not a Function value?", f))
			}

		default:
			// For other pointer-like types, construct a top value corresponding
			// to that type.
			allocateOrSet(l2, allocTopValue(t))
		}
	})

	return L.Elements().AbstractPointerV().UpdatePointer(ptl), mem
}

// Kept for further notice

// When the wildcard corresponds to a pointer type,
// There are two cases based on the type T of the
// underlying pointer.

// In case of a T = *T', then every top allocation site
// must be seeded with its indirect queries, which
// are top represented allocation sites. Each of these
// allocation sites are then top-value injected
// if t2, ok := t.Elem().Underlying().(*T.Pointer); ok {
// 	ptl.ForEach(func(l loc.Location) {
// 		s, _ := l.GetSite()
// 		locs := labelsToAllocs(pt.IndirectQueries[s])
// 		iptl := L.Consts().PointsToNil().Union(locs...)
// 		iptl.FilterNil().ForEach(func(l loc.Location) {
// 			mops.Update(l, L.TopValueForType(t2.Elem()), false)
// 		})
// 		mops.Update(l,
// 			L.Elements().AbstractPointerV().UpdatePointer(iptl).
// 				UpdateAlloc(true), false)
// 	})
// } else {

// In the case of T = T' where T' is not a pointer,
// then injecting top values in the allocation sites
// directly is enough
