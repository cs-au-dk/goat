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

func labelsToAllocs(pt pointer.Pointer) []loc.Location {
	return labelsToLocs(pt, func(l *pointer.Label) loc.Location {
		v, accesses := u.SplitLabel(l)
		var ptr loc.Location
		if global, ok := v.(*ssa.Global); ok {
			ptr = loc.GlobalLocation{Site: global}
		} else {
			ptr = loc.AllocationSiteLocation{
				Goro:    defs.Create().TopGoro(),
				Site:    v,
				Context: v.Parent(),
			}
		}

		if len(accesses) > 0 {
			var typ T.Type
			switch bTyp := v.Type().Underlying().(type) {
			case *T.Slice:
				// Slices are a bit weird in that their allocation sites do not
				// have *T.Pointer type.
				if _, ok := accesses[0].(u.ArrayAccess); !ok {
					log.Fatalln("???", accesses)
				}

				accesses = accesses[1:]
				typ = bTyp.Elem()
				ptr = loc.FieldLocation{
					Base:  ptr,
					Index: -2,
				}

			case *T.Pointer:
				typ = bTyp.Elem()

			default:
				log.Fatalf("Allocation site has unexpected type %T %v", bTyp, bTyp)
			}

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
					ptr = loc.FieldLocation{
						Base:  ptr,
						Index: -2,
					}
				}
			}
		}

		return ptr
	})
}

func labelsToFuncs(pt pointer.Pointer) []loc.Location {
	return labelsToLocs(pt, func(l *pointer.Label) loc.Location {
		if l.Path() != "" {
			log.Fatalln("Non-empty path for label to be turned into function pointer:", l)
		}
		return loc.FunctionPointer{Fun: l.Value().(*ssa.Function)}
	})
}

func allocTopValue(t T.Type) L.AbstractValue {
	switch {
	case utils.IsNamedType(t, "sync", "Mutex"):
		return L.Elements().AbstractMutex().ToTop()
	case utils.IsNamedType(t, "sync", "RWMutex"):
		return L.Elements().AbstractRWMutex().ToTop()
	case utils.IsNamedType(t, "sync", "Cond"):
		return L.Elements().AbstractCond().ToTop()
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

func getAllocationSiteLocation(l loc.Location) loc.AddressableLocation {
	switch l := l.(type) {
	case loc.GlobalLocation:
		return l
	case loc.AllocationSiteLocation:
		return l
	case loc.FieldLocation:
		return getAllocationSiteLocation(l.Base)
	case loc.IndexLocation:
		return getAllocationSiteLocation(l.Base)
	default:
		panic(fmt.Errorf("Cannot retrieve allocation site location from %v", l))
	}
}

// Caches points-to set trees for ssa values so they can quickly be checked for
// equality during the analysis. Before we created a different equivalent tree
// every time the same ssa value was wildcard-swapped.
// Yields a 30-40% speed-up according to (small) experiments.
var swapCache struct {
	pt    *pointer.Result
	cache map[ssa.Value]L.PointsTo
}

// Swap a wildcard value with the result of the upfront analysis.
// Produces a memory where the value has been updated.
func SwapWildcard(pt *pointer.Result, mem L.Memory, l loc.AddressableLocation) L.Memory {
	// Check if swapCache needs to be invalidated
	if pt != swapCache.pt {
		swapCache.pt = pt
		swapCache.cache = map[ssa.Value]L.PointsTo{}
	}

	fset := pt.CallGraph.Root.Func.Prog.Fset

	ssaVal, _ := l.GetSite()
	ptl, found := swapCache.cache[ssaVal]
	if !found {
		// Get all aliases of the given pointer as top allocation sites
		locs := labelsToAllocs(pt.Queries[ssaVal])
		// Construct a points-to set including the nil location and all the top
		// allocation sites.
		ptl = L.Create().Element().PointsTo(locs...).Filter(func(l loc.Location) bool {
			// Get the site of the location
			site, ok := getAllocationSiteLocation(l).GetSite()
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
			typesValid := utils.TypeCompat(ssaVal.Type(), l.Type())
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
				log.Println("Target site:", color.RedString(ssaVal.Name()+" = "+ssaVal.String()))
				log.Println("Target site type: " +
					color.RedString("%s ", ssaVal.Type()) +
					color.CyanString("%p", ssaVal.Type()))
				log.Println("Target site underlying type: " +
					color.RedString("%s ", ssaVal.Type().Underlying()) +
					color.CyanString("%p", ssaVal.Type().Underlying()))
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
				fmt.Println()
			}
			return typesValid
		}).Add(loc.NilLocation{})
		// Nil must be included for soundness

		swapCache.cache[ssaVal] = ptl
	}

	mem = mem.Update(l, L.Elements().AbstractPointerV().UpdatePointer(ptl))

	allocateOrSet := func(key loc.AddressableLocation, value L.AbstractValue) {
		if l, isAllocSite := key.(loc.AllocationSiteLocation); isAllocSite {
			mem = mem.Allocate(l, value, true)
		} else {
			mem = mem.Update(key, value)
		}
	}

	// Only perform case analysis on locations which are not nil
	ptl.FilterNil().ForEach(func(l2_ loc.Location) {
		// l2 might be a FieldLocation, but we need to allocate a value for the base.
		l2 := getAllocationSiteLocation(l2_)
		site, _ := l2.GetSite()
		switch t := site.Type().Underlying().(type) {
		case *T.Pointer:
			// Update every member of the representative points-to set
			// to bind to a top value.
			defer func() {
				if err := recover(); err != nil {
					mops := L.MemOps(mem)
					v, _ := mops.Get(l)
					v2, _ := mops.Get(l2)
					fmt.Println("Original location", l, "has site", ssaVal, "of type", ssaVal.Type())
					fmt.Println("Points to sites: {")
					locs := labelsToAllocs(pt.Queries[ssaVal])
					for _, l := range locs {
						siteVal, _ := getAllocationSiteLocation(l).GetSite()
						fmt.Println(l, "of type", siteVal.Type())
						fmt.Println("Original construct at: ", fset.Position(siteVal.Pos()))
					}
					fmt.Println("Indirectly points-to {")
					for _, l := range pt.IndirectQueries[ssaVal].PointsTo().Labels() {
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
			// For interface, examine the SSA interface allocation site
			// (make interface{} <- x) for the type of x to construct a top
			// value.
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
						l := getAllocationSiteLocation(l_)
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
	return mem
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
