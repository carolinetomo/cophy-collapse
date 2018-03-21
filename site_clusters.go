package cophymaru

import (
	"bytes"
	"fmt"
	"math/rand"
	"strconv"
)

//InitializeClusters will initialize branch length clusters for the cluster model
func InitializeClusters(chain *MCMC) {
	ntraits := len(chain.TREE.CONTRT)
	K := ntraits / 100
	if K < 1 {
		K = 5
	}
	var uniqueK []int
	for k := 0; k < K; k++ {
		uniqueK = append(uniqueK, k)
		//break
	}
	chain.UNIQUEK = uniqueK
	for i := 0; i < int(chain.NSITES); i++ {
		clusterAssignment := uniqueK[rand.Intn(len(uniqueK))]
		if i == 0 {
			fmt.Println("site", i, clusterAssignment)
		}
		chain.CLUS = append(chain.CLUS, clusterAssignment)
		if _, ok := chain.CLUSTERSET[clusterAssignment]; ok {
			chain.CLUSTERSET[clusterAssignment] = append(chain.CLUSTERSET[clusterAssignment], i)
		} else {
			var curfill []int
			curfill = append(curfill, i)
			chain.CLUSTERSET[clusterAssignment] = curfill
		}
	}
	for _, n := range chain.NODES {
		n.ClustLEN = make(map[int]float64)
		for i := 0; i < K; i++ {
			n.ClustLEN[i] = rand.ExpFloat64() //n.LEN //append(n.ClustLEN[i], n.LEN)
		}
	}
	return
}

//ClusterString will return a string of the current set of clusters
func ClusterString(cSet map[int][]int) string {
	var buffer bytes.Buffer
	for c := range cSet {
		buffer.WriteString("(")
		for ind, site := range cSet[c] {
			cur := strconv.Itoa(site)
			buffer.WriteString(cur)
			stop := len(cSet[c]) - 1
			if ind != stop {
				buffer.WriteString(",")
			}
		}
		buffer.WriteString(");")
	}
	return buffer.String()
}

/*/SiteDistMatrix will calculate the distance matrix at each site
func SiteDistMatrix(nodes []*Node) {
	for i, ni := range nodes {
		if len(ni.CHLD) != 0 {
			continue
		}
		for j, nj := range nodes {
			if len(ni.CHLD) != 0 {
				continue
			} else if i == j{

			}
		}
	}
}
*/

//StartingSiteLen makes starting branch lengths for each site for clustering.
func StartingSiteLen(chain *MCMC) {
	for i := 0; i < int(chain.NSITES); i++ {
		chain.CLUS = append(chain.CLUS, i)
		if _, ok := chain.CLUSTERSET[i]; ok {
			chain.CLUSTERSET[i] = append(chain.CLUSTERSET[i], i)
		} else {
			var curfill []int
			curfill = append(curfill, i)
			chain.CLUSTERSET[i] = curfill
		}
	}
	chain.UNIQUEK = chain.CLUS
	for _, n := range chain.NODES {
		n.ClustLEN = make(map[int]float64)
		for i := 0; i < len(chain.CLUS); i++ {
			n.ClustLEN[i] = Rexp(10.) //rand.Float64() //n.LEN //append(n.ClustLEN[i], n.LEN)
		}
	}
}

//AssignClustLens will assign the lengths associated with a particular cluster to the branch lengths
func AssignClustLens(chain *MCMC, cluster int) {
	for _, n := range chain.NODES {
		n.LEN = n.ClustLEN[cluster]
	}
}

//SiteCluster is a struct for storing clusters of branch lengths
type SiteCluster struct {
	LABEL int
	SITES int
}

//ClusterSet will store all of the individual site clusters
type ClusterSet struct {
	Clusters   []*SiteCluster
	UniqueK    []int
	SiteVector []int
	ALPHA      float64
	ALPHAPROB  float64
}
