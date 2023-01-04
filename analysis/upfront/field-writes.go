package upfront

import (
	"sort"

	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/graph"

	"fmt"
	T "go/types"
	"log"
	"strings"

	"github.com/fatih/color"
	"golang.org/x/tools/container/intsets"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"
)

/* TODO: We can increase precision by giving fields in embedded structs unique indices.
Then we can say that only field f is written in the following program, instead of s as a whole:
type A = struct { s struct { f, g int } }
func w(a *A) { a.s.f = 10 }
*/

// WrittenFields computes the sets of heap locations that may be written to
// in a specific DAG component. Each DAG component has a unique identifier
// which at which a specific set is bound.
type WrittenFields struct {
	typMap          typeutil.Map
	callDAG         graph.SCCDecomposition[*ssa.Function]
	writtenFields   []intsets.Sparse
	writtenMaps     []utils.SSAValueSet
	writtenSlices   []utils.SSAValueSet
	writtenPointers []utils.SSAValueSet
}

func (w WrittenFields) FieldInfo(typ *T.Struct, funs map[*ssa.Function]struct{}) (
	isWritten func(fieldIndex int) bool,
) {
	if startIndex_itf := w.typMap.At(typ); startIndex_itf == nil {
		return func(int) bool { return false }
	} else {
		var wf intsets.Sparse
		for fun := range funs {
			if compIdx := w.callDAG.ComponentOf(fun); compIdx != -1 {
				wf.UnionWith(&w.writtenFields[compIdx])
			}
		}

		startIndex := startIndex_itf.(int)
		return func(fieldIndex int) bool {
			return wf.Has(startIndex + fieldIndex)
		}
	}
}

func (w WrittenFields) combinedInfo(
	funs map[*ssa.Function]struct{},
	sets []utils.SSAValueSet) (
	isUpdated func(v ssa.Value) bool,
) {
	set := utils.MakeSSASet()

	for fun := range funs {
		if compIdx := w.callDAG.ComponentOf(fun); compIdx != -1 {
			set = set.Join(sets[compIdx])
		}
	}

	return set.Contains
}

func (w WrittenFields) MapCombinedInfo(funs map[*ssa.Function]struct{}) (
	isUpdated func(v ssa.Value) bool,
) {
	return w.combinedInfo(funs, w.writtenMaps)
}
func (w WrittenFields) SliceCombinedInfo(funs map[*ssa.Function]struct{}) (
	isUpdated func(v ssa.Value) bool,
) {
	return w.combinedInfo(funs, w.writtenSlices)
}
func (w WrittenFields) PointerCombinedInfo(funs map[*ssa.Function]struct{}) (
	isUpdated func(v ssa.Value) bool,
) {
	return w.combinedInfo(funs, w.writtenPointers)
}

func (w WrittenFields) IsFieldWrittenFromFunction(fun *ssa.Function, typ *T.Struct, fieldIndex int) bool {
	compIdx := w.callDAG.ComponentOf(fun)
	if compIdx == -1 {
		return false
	}

	if indexOfFirstField_itf := w.typMap.At(typ); indexOfFirstField_itf == nil {
		return false
	} else {
		return w.writtenFields[compIdx].Has(indexOfFirstField_itf.(int) + fieldIndex)
	}
}

func ComputeWrittenFields(pt *PointerResult, callDAG graph.SCCDecomposition[*ssa.Function]) WrittenFields {
	components := callDAG.Components
	if len(components) == 0 || len(components[0]) == 0 {
		panic("Empty call DAG provided to side-effect analysis")
	}

	typMap := typeutil.Map{}
	fieldIndex := 0

	getStartIndex := func(structT *T.Struct) int {
		if v := typMap.At(structT); v != nil {
			return v.(int)
		}

		typMap.Set(structT, fieldIndex)
		fieldIndex += structT.NumFields()
		return fieldIndex - structT.NumFields()
	}

	ncomponents := len(components)
	writtenFields := make([]intsets.Sparse, ncomponents)
	writtenMaps := make([]utils.SSAValueSet, ncomponents)
	writtenSlices := make([]utils.SSAValueSet, ncomponents)
	writtenPointers := make([]utils.SSAValueSet, ncomponents)

	for i, component := range components {
		fieldSet := &writtenFields[i]
		maps := &writtenMaps[i]
		slices := &writtenSlices[i]
		pointers := &writtenPointers[i]
		*maps = utils.MakeSSASet()
		*slices = utils.MakeSSASet()
		*pointers = utils.MakeSSASet()
		for _, fun := range component {
			for _, block := range fun.Blocks {
				for _, insn := range block.Instrs {
					if mapup, ok := insn.(*ssa.MapUpdate); ok {
						for _, label := range pt.Queries[mapup.Map].PointsTo().Labels() {
							if val := label.Value(); val != nil {
								*maps = (*maps).Add(val)
							}
						}
					}
					if store, ok := insn.(*ssa.Store); ok {
						for _, label := range pt.Queries[store.Addr].PointsTo().Labels() {
							allocSite, accesses := SplitLabel(label)
							if allocSite == nil {
								continue
							}

							if len(accesses) > 0 {
								if _, ok := accesses[0].(ArrayAccess); ok {
									*slices = (*slices).Add(allocSite)
								}
							} else if alloc, ok := allocSite.(*ssa.Alloc); !ok || alloc.Heap {
								*pointers = (*pointers).Add(allocSite)
							}

							ptr, ok := allocSite.Type().Underlying().(*T.Pointer)
							if !ok {
								continue
							}

							structT, ok := ptr.Elem().Underlying().(*T.Struct)
							if !ok {
								continue
							}

							// Field writes during composite literal initialization cannot
							// have side-effects on parameters / free variables.
							// We filter out such cases here:
							if faddr, ok := store.Addr.(*ssa.FieldAddr); ok &&
								// The FieldAddr references the allocation site directly:
								faddr.X == allocSite {
								continue
							}

							// Writing a plain struct into a pointer is equivalent
							// to writing all the fields.
							if wStructT, ok := store.Val.Type().Underlying().(*T.Struct); ok {
								startIndex := getStartIndex(wStructT)
								for fi := 0; fi < wStructT.NumFields(); fi++ {
									fieldSet.Insert(startIndex + fi)
								}
							} else if len(accesses) > 0 {
								// Check if we are storing into a field.
								if fieldAccess, ok := accesses[0].(FieldAccess); ok {
									firstField := fieldAccess.Field
									fieldIndex := -1
									for fi := 0; fi < structT.NumFields(); fi++ {
										field := structT.Field(fi)
										if field.Name() == firstField {
											fieldIndex = fi
											break
										}
									}

									if fieldIndex == -1 {
										fmt.Println(structT, label.Path())
										log.Fatalf("None of %v's fields has name: %s", structT, firstField)
									}

									fieldSet.Insert(getStartIndex(structT) + fieldIndex)
								}
							}
						}
					}
				}
			}

			for _, edge := range callDAG.Original.Edges(fun) {
				if ncomp := callDAG.ComponentOf(edge); ncomp != i {
					fieldSet.UnionWith(&writtenFields[ncomp])
					*maps = (*maps).Join(writtenMaps[ncomp])
					*slices = (*slices).Join(writtenSlices[ncomp])
					*pointers = (*pointers).Join(writtenPointers[ncomp])
				}
			}
		}
	}

	return WrittenFields{
		typMap, callDAG,
		writtenFields,
		writtenMaps,
		writtenSlices,
		writtenPointers,
	}
}

func (w WrittenFields) String() string {
	var b strings.Builder
	b.WriteString("WrittenFields analysis result:\n")

	for i, component := range w.callDAG.Components {
		var headers struct {
			component, field, mp, slice, pointer bool
		}
		header := func() {
			if !headers.component {
				headers.component = true
				funNames := make([]string, len(component))
				for j, fun := range component {
					funNames[j] = fun.String()
				}
				sort.Strings(funNames)

				fmt.Fprintf(&b, "\nIn component of:\n%s\n", color.YellowString(strings.Join(funNames, "\n")))
			}
		}
		sectionHeader := func(written *bool, msg string) {
			header()
			if !*written {
				*written = true
				b.WriteString(msg)
			}
		}
		fieldHeader := func() {
			sectionHeader(&headers.field, color.GreenString("The following fields are written:\n"))
		}
		mapHeader := func() {
			sectionHeader(&headers.mp, color.BlueString("The following maps are updated:\n"))
		}
		sliceHeader := func() {
			sectionHeader(&headers.slice, color.CyanString("The following slices (array pointers) are updated:\n"))
		}
		pointerHeader := func() {
			sectionHeader(&headers.pointer, color.RedString("The following pointers are updated:\n"))
		}

		wf := &w.writtenFields[i]
		w.typMap.Iterate(func(typ T.Type, firstIdx_itf any) {
			firstIdx := firstIdx_itf.(int)
			fields := []string{}
			structT := typ.(*T.Struct)
			for fi := 0; fi < structT.NumFields(); fi++ {
				field := structT.Field(fi)
				if wf.Has(firstIdx + fi) {
					fields = append(fields, fmt.Sprintf("%d %s", fi, field.Name()))
				}
			}

			if len(fields) != 0 {
				fieldHeader()

				fmt.Fprintf(&b, "\t%v:\n\t\t%s\n", structT, strings.Join(fields, "\n\t\t"))
			}
		})

		wmp := w.writtenMaps[i]
		if size := wmp.Size(); size > 0 {
			mapHeader()
			fmt.Fprintf(&b, "%s\n", wmp)
		}
		wslc := w.writtenSlices[i]
		if size := wslc.Size(); size > 0 {
			sliceHeader()
			fmt.Fprintf(&b, "%s\n", wslc)
		}
		wptr := w.writtenPointers[i]
		if size := wptr.Size(); size > 0 {
			pointerHeader()
			fmt.Fprintf(&b, "%s\n", wptr)
		}
		fmt.Fprintf(&b, "\n")
	}

	return b.String()
}
