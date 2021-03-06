package cophycollapse

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"math"
	"math/big"
	"math/rand"
	"os"
	"strconv"

	"gonum.org/v1/gonum/mat"
)

//UVNDPPGibbs is a struct to store the attributes of a collapsed DPP gibbs sampler for the BM phylo gene expression data
type UVNDPPGibbs struct {
	Dist            *DistanceMatrix
	Clusters        map[int]*UVNLike
	SiteAssignments map[int]int
	Prior           *NormalGammaPrior
	Gen             int
	PrintFreq       int
	WriteFreq       int
	Threads         int
	Workers         int
	Alpha           float64
	ClustOutFile    string
	LogOutFile      string
}

//ClusterString will return a string of the current set of clusters
func (chain *UVNDPPGibbs) ClusterString() string {
	var buffer bytes.Buffer
	cSet := chain.Clusters
	for _, like := range cSet {
		buffer.WriteString("(")
		for ind, site := range like.Sites {
			cur := strconv.Itoa(site)
			buffer.WriteString(cur)
			stop := len(like.Sites) - 1
			if ind != stop {
				buffer.WriteString(",")
			}
		}
		buffer.WriteString(");")
	}
	return buffer.String()
}

//InitUVNGibbs will initialize the parameters for the collapsed gibbs sampler.
func InitUVNGibbs(nodes []*Node, prior *NormalGammaPrior, gen, print, write, threads, workers int, alpha float64, treeOut, logOut string) *UVNDPPGibbs {
	gibbs := new(UVNDPPGibbs)
	dist := DM(nodes)
	gibbs.Dist = dist
	gibbs.Prior = prior
	gibbs.Gen = gen
	gibbs.Alpha = alpha
	gibbs.PrintFreq = print
	gibbs.WriteFreq = write
	gibbs.Threads = threads
	gibbs.Workers = workers
	gibbs.ClustOutFile = treeOut
	gibbs.LogOutFile = logOut
	gibbs.startingClusters()
	return gibbs
}

//Run will run MCMC simulations
func (chain *UVNDPPGibbs) Run() {
	//var assoc []*mat.Dense
	f, err := os.Create(chain.ClustOutFile)
	if err != nil {
		log.Fatal(err)
	}
	w := bufio.NewWriter(f)
	logFile, err := os.Create(chain.LogOutFile)
	if err != nil {
		log.Fatal(err)
	}
	logWriter := bufio.NewWriter(logFile)

	matStrings := make(map[*mat.Dense]string)
	var KC []int
	for gen := 0; gen <= chain.Gen; gen++ {
		chain.collapsedGibbsClusterUpdate()
		clusterString := chain.ClusterString()
		//fmt.Println(len(chain.Clusters), clusterString)
		m := chain.clusterAssociationMatrix()
		//matPrint(m)
		matStrings[m] = clusterString
		//assoc = append(assoc, m)
		KC = append(KC, len(chain.Clusters))
		if gen%chain.PrintFreq == 0 {
			fmt.Println(gen, len(chain.Clusters))
		}
		if gen%chain.WriteFreq == 0 {
			fmt.Fprint(logWriter, strconv.Itoa(len(chain.Clusters))+"\n")
			fmt.Fprint(w, clusterString+"\n")
		}
	}
	logWriter.Flush()
	err = w.Flush()
	if err != nil {
		log.Fatal(err)
	}
	f.Close()
	mapK := MeanInt(KC)
	fmt.Println("MEAN NUMBER OF CLUSTERS:", mapK)
	chain.summarizeMAPBestK(matStrings, mapK)
}

//this will take the MAP clustering given the mean # of clusters
func (chain *UVNDPPGibbs) summarizeMAPBestK(matStrings map[*mat.Dense]string, K int) map[*mat.Dense]int {
	counts := make(map[*mat.Dense]int)
	//var seen []*mat.Dense
	//seen = append(seen, assoc[0])
	start := false
	var curK int
	for m, st := range matStrings {
		curK = 0
		for _, char := range st {
			if char == 59 { //count number of semicolons (ASCII character 59) to get the number of clusters
				curK++
			}
		}
		if curK != K {
			continue
		}
		if start == false {
			//seen = append(seen, m)
			counts[m] = 1
			start = true
			continue
		}
		unique := true
		for checkMat := range counts {
			equal := mat.Equal(m, checkMat)
			if equal == true {
				counts[checkMat]++
				//num++
				unique = false
				break
			}
		}
		if unique == true {
			counts[m] = 1
		}
	}
	freq := 0
	var MAP string
	for i, num := range counts {
		clusterString := matStrings[i]
		//if num != 1 {
		//	fmt.Println(i, clusterString, num)
		//}
		if num > freq {
			MAP = clusterString
			freq = num
		}
	}
	fmt.Println(MAP, freq)
	return counts
}

func (chain *UVNDPPGibbs) summarizeMAP(matStrings map[*mat.Dense]string) map[*mat.Dense]int {
	counts := make(map[*mat.Dense]int)
	//var seen []*mat.Dense
	//seen = append(seen, assoc[0])
	start := false
	for m := range matStrings {
		if start == false {
			//seen = append(seen, m)
			counts[m] = 1
			start = true
			continue
		}
		unique := true
		for checkMat := range counts {
			equal := mat.Equal(m, checkMat)
			if equal == true {
				counts[checkMat]++
				//num++
				unique = false
				break
			}
		}
		if unique == true {
			counts[m] = 1
		}
	}
	freq := 0
	var MAP string
	for i, num := range counts {
		clusterString := matStrings[i]
		//if num != 1 {
		//	fmt.Println(i, clusterString, num)
		//}
		if num > freq {
			MAP = clusterString
			freq = num
		}
	}
	fmt.Println(MAP, freq)
	return counts
}

/*
func (chain *UVNDPPGibbs) summarize(assoc []*mat.Dense) {
	counts := make(map[fmt.Formatter]int)
	for _, gen := range assoc {
		rows := mat.Formatted(gen, mat.Prefix(""), mat.Squeeze())
		if _, ok := counts[rows]; ok {
			counts[rows]++
		} else {
			counts[rows] = 1
		}
	}
	fmt.Println(counts)
}
*/

func (chain *UVNDPPGibbs) clusterAssociationMatrix() *mat.Dense {
	mat := mat.NewDense(chain.Dist.IntNSites, chain.Dist.IntNSites, nil)
	for site1 := 0; site1 < chain.Dist.IntNSites; site1++ {
		for site2 := 0; site2 < chain.Dist.IntNSites; site2++ {
			//if site1 == site2 || site1 > site2 {
			//	continue
			//}
			if chain.SiteAssignments[site1] == chain.SiteAssignments[site2] {
				mat.Set(site1, site2, 1.)
			} else {
				mat.Set(site1, site2, 0.)
			}
		}
	}
	return mat
}

func (chain *UVNDPPGibbs) collapsedGibbsClusterUpdate() {
	for k, v := range chain.SiteAssignments {
		chain.collapsedSiteClusterUpdate(k, v)
	}
}

//this will 'reseat' site i
func (chain *UVNDPPGibbs) collapsedSiteClusterUpdate(site int, siteClusterLab int) {
	siteCluster := chain.Clusters[siteClusterLab]
	if len(siteCluster.Sites) == 1 { // delete the cluster containing the current site if it is a singleton
		delete(chain.Clusters, siteClusterLab)
	} else { // delete current site from its cluster
		var newSlice []int
		for _, s := range siteCluster.Sites {
			if s != site {
				newSlice = append(newSlice, s)
			}
		}
		siteCluster.Sites = newSlice
	}
	sum := big.NewFloat(0.)
	clustProbs := make(map[int]*big.Float)
	for k, v := range chain.Clusters {
		prob := chain.calcClusterProb(k, v, site)
		//fmt.Println(prob)
		//sum += prob
		sum.Add(sum, prob)
		clustProbs[k] = prob
	}
	newClustLab := MaxClustLabBig(clustProbs) + 1
	g0 := chain.calcNewClusterProb(site)
	newClustProb := big.NewFloat(chain.Alpha / (chain.Dist.NSites + chain.Alpha - 1.) * g0) //chain.Prior.PPDensity
	clustProbs[newClustLab] = newClustProb
	sum.Add(sum, newClustProb)
	r := rand.Float64()
	cumprob := 0.
	newCluster := -1
	for k, v := range clustProbs {
		//clustProbs[k] = v / sum
		quo := new(big.Float).Quo(v, sum)
		cur, _ := quo.Float64()
		cumprob += cur
		if cumprob > r {
			newCluster = k
			break
		}
	}
	if newCluster < 0 {
		fmt.Println("couldn't reseat site", site)
		os.Exit(0)
	}
	//fmt.Println(newCluster)
	if newCluster != newClustLab {
		addK := chain.Clusters[newCluster]
		addK.Sites = append(addK.Sites, site)
		chain.SiteAssignments[site] = newCluster
		chain.updateNewClusterPosterior(addK, site)
	} else {
		chain.makeNewCluster(site, newCluster)
	}

	//os.Exit(0)
}

func (chain *UVNDPPGibbs) makeNewCluster(site int, newLab int) {
	cur := new(UVNLike)
	cur.Sites = []int{site}
	siteDist := chain.Dist.MatSites[site]
	for _, j := range siteDist {
		cur.KN = append(cur.KN, chain.Prior.K0+1.)
		cur.MuN = append(cur.MuN, (chain.Prior.Mu0 * 1. * math.Pow(j-chain.Prior.Mu0, 2.) / (2. * (chain.Prior.K0 + 1.))))
		cur.AlphaN = append(cur.AlphaN, chain.Prior.Alpha0+0.5)
		cur.BetaN = append(cur.BetaN, chain.Prior.Beta0+(chain.Prior.K0*1.*math.Pow(chain.Prior.Mu0-j, 2.))/(2.*chain.Prior.K0+1.))
		cur.DataSum = append(cur.DataSum, j)
	}
	chain.Clusters[newLab] = cur
	chain.SiteAssignments[site] = newLab
}

func (chain *UVNDPPGibbs) updateNewClusterPosterior(addK *UVNLike, site int) {
	siteDist := chain.Dist.MatSites[site]
	newKLen := float64(len(addK.Sites))
	for i, d := range siteDist {
		kN := addK.KN[i]
		muN := addK.MuN[i]
		betaN := addK.BetaN[i]
		addK.AlphaN[i] += 0.5
		addK.KN[i] += 1.
		addK.DataSum[i] += d
		sampMean := addK.DataSum[i] / newKLen
		addK.MuN[i] = (chain.Prior.K0 * chain.Prior.Mu0) + (newKLen*sampMean)/(chain.Prior.K0+newKLen)
		addK.BetaN[i] = betaN + ((kN * math.Pow(d-muN, 2)) / (2 * (kN + 1.)))
		//TODO: fix BetaN update ^
	}
}

func (chain *UVNDPPGibbs) calcNewClusterProb(testsite int) (pp float64) {
	pp = 1.
	for _, trait := range chain.Dist.MatSites[testsite] {
		pp *= chain.Prior.PPDensity.Prob(trait)
	}
	return
}

func (chain *UVNDPPGibbs) calcClusterProb(clusLab int, clust *UVNLike, testsite int) *big.Float {
	var knPlus1, betaPlus1, alphaPlus1 float64
	//pp := 1.
	bigpp := big.NewFloat(1.)
	for i, trait := range chain.Dist.MatSites[testsite] {
		alphaN := clust.AlphaN[i]
		kN := clust.KN[i]
		betaN := clust.BetaN[i]
		alphaPlus1 = alphaN + 0.5
		knPlus1 = kN + 1.
		betaPlus1 = betaN + ((kN * math.Pow(trait-clust.MuN[i], 2)) / (2 * (knPlus1)))
		q1t, s := math.Lgamma(alphaPlus1)
		q1t = q1t * float64(s)
		q1b, s := math.Lgamma(clust.AlphaN[i])
		q1b = q1b * float64(s)
		q1 := q1t - q1b
		q2t := math.Pow(betaN, alphaN)
		q2b := math.Pow(betaPlus1, alphaPlus1)
		q2 := q2t / q2b
		q3 := math.Sqrt(kN / knPlus1)
		//bq1 := big.NewFloat(q1)
		//bq2 := big.NewFloat(q2)
		//bq3 := big.NewFloat(q3)
		pp := q1 + math.Log(q2) + math.Log(q3) + 35.92242295470006
		//fmt.Println(math.Exp(pp))
		//os.Exit(0)
		//bigpp.Mul(bigpp, bq1)
		//bigpp.Mul(bigpp, bq2)
		//bigpp.Mul(bigpp, bq3)
		bigpp.Mul(bigpp, big.NewFloat(math.Exp(pp)))
		//muPlus1 = append(muPlus1, clust.MuN[i])
		//TODO: need to finish typing the calculation of these parameters and calc the PP density
	}
	//fmt.Println(bigpp, pp)
	//os.Exit(0)

	return bigpp
}

func (chain *UVNDPPGibbs) startingClusters() {
	clus := make(map[int]*UVNLike)
	lab := 0
	siteClust := make(map[int]int)
	for k, v := range chain.Dist.MatSites {
		cur := new(UVNLike)
		cur.Sites = []int{k}
		for _, j := range v {
			cur.KN = append(cur.KN, chain.Prior.K0+1.)
			cur.MuN = append(cur.MuN, (chain.Prior.Mu0 * 1. * math.Pow(j-chain.Prior.Mu0, 2.) / (2. * (chain.Prior.K0 + 1.))))
			cur.AlphaN = append(cur.AlphaN, chain.Prior.Alpha0+0.5)
			cur.BetaN = append(cur.BetaN, chain.Prior.Beta0+(chain.Prior.K0*1.*math.Pow(chain.Prior.Mu0-j, 2.))/(2.*chain.Prior.K0+1.))
			cur.DataSum = append(cur.DataSum, j)
		}
		clus[lab] = cur
		siteClust[k] = lab
		lab++
	}
	chain.Clusters = clus
	chain.SiteAssignments = siteClust
}
