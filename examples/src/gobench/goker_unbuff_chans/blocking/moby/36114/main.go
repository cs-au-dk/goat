/*
 * Project: moby
 * Issue or PR  : https://github.com/moby/moby/pull/36114
 * Buggy version: 6d4d3c52ae7c3f910bfc7552a2a673a8338e5b9f
 * fix commit-id: a44fcd3d27c06aaa60d8d1cbce169f0d982e74b1
 * Flaky: 100/100
 * Description:
 *   This is a double lock bug. The the lock for the
 * struct svm has already been locked when calling
 * svm.hotRemoveVHDsAtStart()
 */
package main

type serviceVM chan bool

func (svm *serviceVM) hotAddVHDsAtStart() {
	*svm <- true
	defer func() { <-*svm }()
	svm.hotRemoveVHDsAtStart()
}

func (svm *serviceVM) hotRemoveVHDsAtStart() {
	*svm <- true // Double lock here
	defer func() { <-*svm }()
}

func main() {
	s := new(serviceVM)
	*s = make(chan bool)
	go s.hotAddVHDsAtStart()
}
