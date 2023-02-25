package routing

import (
	"encoding/json"
	"math"
	"math/rand"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
)

func TestTableJSONRoundTrip(t *testing.T) {
	const (
		host = "testhost"
		ns   = "testns"
	)
	r := require.New(t)
	tbl := NewTable()
	tgt := NewTarget(
		ns,
		"testsvc",
		8082,
		"testdepl",
		1234,
	)
	r.NoError(tbl.AddTarget(host, tgt))

	b, err := json.Marshal(&tbl)
	r.NoError(err)

	returnTbl := NewTable()
	r.NoError(json.Unmarshal(b, returnTbl))
	retTarget, err := returnTbl.Lookup(host)
	r.NoError(err)
	r.Equal(tgt.Service, retTarget.Service)
	r.Equal(tgt.Port, retTarget.Port)
	r.Equal(tgt.Deployment, retTarget.Deployment)
}

func TestTableRemove(t *testing.T) {
	const (
		host = "testrm"
		ns   = "testns"
	)

	r := require.New(t)
	tgt := NewTarget(
		ns,
		"testrm",
		8084,
		"testrmdepl",
		1234,
	)

	tbl := NewTable()

	// add the target to the table and ensure that you can look it up
	r.NoError(tbl.AddTarget(host, tgt))
	retTgt, err := tbl.Lookup(host)
	r.Equal(&tgt, retTgt)
	r.NoError(err)

	// remove the target and ensure that you can't look it up
	r.NoError(tbl.RemoveTarget(host))
	retTgt, err = tbl.Lookup(host)
	r.Equal((*Target)(nil), retTgt)
	r.Equal(ErrTargetNotFound, err)
}

func TestTableReplace(t *testing.T) {
	const ns = "testns"
	r := require.New(t)
	const host1 = "testreplhost1"
	const host2 = "testreplhost2"
	tgt1 := NewTarget(
		ns,
		"tgt1",
		9090,
		"depl1",
		1234,
	)
	tgt2 := NewTarget(
		ns,
		"tgt2",
		9091,
		"depl2",
		1234,
	)
	// create two routing tables, each with different targets
	tbl1 := NewTable()
	r.NoError(tbl1.AddTarget(host1, tgt1))
	tbl2 := NewTable()
	r.NoError(tbl2.AddTarget(host2, tgt2))

	// replace the second table with the first and ensure that the tables
	// are now equal
	tbl2.Replace(tbl1)

	r.Equal(tbl1, tbl2)
}

var _ = Describe("Table", func() {
	Describe("Lookup", func() {
		var (
			tltcs = newTableLookupTestCases(5)
			table = NewTable()
		)

		Context("with new port-agnostic configuration", func() {
			BeforeEach(func() {
				for _, tltc := range tltcs {
					err := table.AddTarget(tltc.HostWithoutPort(), tltc.Target())
					Expect(err).NotTo(HaveOccurred())
				}
			})

			AfterEach(func() {
				for _, tltc := range tltcs {
					err := table.RemoveTarget(tltc.HostWithoutPort())
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should return correct target for host without port", func() {
				for _, tltc := range tltcs {
					target, err := table.Lookup(tltc.HostWithoutPort())
					Expect(err).NotTo(HaveOccurred())
					Expect(target).To(HaveValue(Equal(tltc.Target())))
				}
			})

			It("should return correct target for host with port", func() {
				for _, tltc := range tltcs {
					target, err := table.Lookup(tltc.HostWithPort())
					Expect(err).NotTo(HaveOccurred())
					Expect(target).To(HaveValue(Equal(tltc.Target())))
				}
			})
		})

		Context("with legacy port-specific configuration", func() {
			BeforeEach(func() {
				for _, tltc := range tltcs {
					err := table.AddTarget(tltc.HostWithPort(), tltc.Target())
					Expect(err).NotTo(HaveOccurred())
				}
			})

			AfterEach(func() {
				for _, tltc := range tltcs {
					err := table.RemoveTarget(tltc.HostWithPort())
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should error for host without port", func() {
				for _, tltc := range tltcs {
					target, err := table.Lookup(tltc.HostWithoutPort())
					Expect(err).To(MatchError(ErrTargetNotFound))
					Expect(target).To(BeNil())
				}
			})

			It("should return correct target for host with port", func() {
				for _, tltc := range tltcs {
					target, err := table.Lookup(tltc.HostWithPort())
					Expect(err).NotTo(HaveOccurred())
					Expect(target).To(HaveValue(Equal(tltc.Target())))
				}
			})
		})
	})
})

type tableLookupTestCase struct {
	target Target
}

func newTableLookupTestCase() tableLookupTestCase {
	target := NewTarget(
		strconv.Itoa(rand.Int()),
		strconv.Itoa(rand.Int()),
		rand.Intn(math.MaxUint16),
		strconv.Itoa(rand.Int()),
		int32(rand.Intn(math.MaxUint8)),
	)

	return tableLookupTestCase{
		target: target,
	}
}

func (tltc tableLookupTestCase) Target() Target {
	return tltc.target
}

func (tltc tableLookupTestCase) HostWithoutPort() string {
	return tltc.target.Service + "." + tltc.target.Namespace + ".svc.cluster.local"
}

func (tltc tableLookupTestCase) HostWithPort() string {
	return tltc.HostWithoutPort() + ":" + strconv.Itoa(tltc.target.Port)
}

type tableLookupTestCases []tableLookupTestCase

func newTableLookupTestCases(count uint) tableLookupTestCases {
	tltcs := make(tableLookupTestCases, count)
	for i := uint(0); i < count; i++ {
		tltcs[i] = newTableLookupTestCase()
	}
	return tltcs
}
