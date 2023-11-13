package merkle

import (
	"fmt"
	"math"
	"math/big"

	"github.com/holiman/uint256"
	"github.com/iden3/go-iden3-crypto/ff"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"golang.org/x/crypto/sha3"
)

type TreeNode struct {
	Value *uint256.Int
}

func (n *TreeNode) UnmarshalText(text []byte) error {
	var err error
	n.Value, err = uint256.FromDecimal(string(text))

	return err
}

func (n TreeNode) MarshalText() (text []byte, err error) {
	return []byte(n.Value.Dec()), nil
}

type Tree struct {
	Nodes []TreeNode
}

type Proof struct {
	Path []TreeNode `json:"path"`

	// Indices represents a bitmask where each bit, starting from the rightmost bit,
	// tells whether the corresponding Path node is a left child (0)
	// or a right child (1) in the original Merkle tree.
	Indices int `json:"indices"`
}

var EmptyLeafValue = new(uint256.Int).Mod(
	uint256.MustFromBig(new(big.Int).SetBytes(makeSeedForEmptyLeaf())),
	uint256.MustFromBig(ff.Modulus()),
)

func makeSeedForEmptyLeaf() []byte {
	hash := sha3.NewLegacyKeccak256()
	hash.Write([]byte("Galactica"))

	return hash.Sum(nil)
}

const TreeDepth = 32

func HashFunc(input []*big.Int) (*big.Int, error) {
	return poseidon.Hash(input)
}

func NewEmptyTree(depth int, leafValue *uint256.Int) (*Tree, error) {
	if depth < 1 {
		return nil, fmt.Errorf("invalid tree depth")
	}

	nodes := make([]TreeNode, 1<<depth-1)

	firstNodeIndex := len(nodes)

	for i, nodesAmount := 0, 1<<(depth-1); i < depth; i, nodesAmount = i+1, nodesAmount/2 {
		firstNodeIndex -= nodesAmount

		for j := 0; j < nodesAmount; j++ {
			nodes[firstNodeIndex+j].Value = leafValue
		}

		if firstNodeIndex > 0 {
			node, err := computeChildrenHash(firstNodeIndex-1, nodes)
			if err != nil {
				return nil, fmt.Errorf("compute hash: %w", err)
			}

			leafValue = node.Value
		}
	}

	return &Tree{
		Nodes: nodes,
	}, nil
}

func (t *Tree) SetLeaf(i int, val TreeNode) error {
	leavesAmount := t.GetLeavesAmount()

	if i >= leavesAmount || i < 0 {
		return fmt.Errorf("invalid leaf index")
	}

	j := len(t.Nodes) - leavesAmount + i
	t.Nodes[j] = val

	for j := GetParentIndex(j); j > 0; j = GetParentIndex(j) {
		var err error
		t.Nodes[j], err = computeChildrenHash(j, t.Nodes)
		if err != nil {
			return fmt.Errorf("compute hash: %w", err)
		}
	}

	var err error
	t.Nodes[0], err = computeChildrenHash(0, t.Nodes)
	if err != nil {
		return fmt.Errorf("compute hash: %w", err)
	}

	return nil
}

func (t *Tree) GetProof(i int) (Proof, error) {
	leavesAmount := t.GetLeavesAmount()

	if i >= leavesAmount || i < 0 {
		return Proof{}, fmt.Errorf("invalid leaf index")
	}

	proof := Proof{
		Path: make([]TreeNode, int(math.Log2(float64(len(t.Nodes)+1)))),
	}

	j := len(t.Nodes) - leavesAmount + i

	for level := 0; j > 0; j, level = GetParentIndex(j), level+1 {
		proof.Indices |= j % 2 << level

		siblingIndex := GetSiblingIndex(j)
		sibling := t.Nodes[siblingIndex]

		proof.Path[level] = sibling
	}

	proof.Path[len(proof.Path)-1] = t.Root()

	return proof, nil
}

func (t *Tree) Root() TreeNode {
	return t.Nodes[0]
}

func (t *Tree) GetLeavesAmount() int {
	return (len(t.Nodes) + 1) / 2
}

func GetParentIndex(i int) int {
	return (i - 1) / 2
}

func GetSiblingIndex(i int) int {
	if IsRightChild(i) {
		return i - 1
	}

	return i + 1
}

func IsRightChild(i int) bool {
	return i%2 == 0
}

func computeChildrenHash(i int, nodes []TreeNode) (TreeNode, error) {
	return computeNodeHash(getChildrenOf(i, nodes))
}

func computeNodeHash(left, right TreeNode) (TreeNode, error) {
	val, err := HashFunc([]*big.Int{left.Value.ToBig(), right.Value.ToBig()})
	if err != nil {
		return TreeNode{}, err
	}

	convertedVal, isOverflow := uint256.FromBig(val)
	if isOverflow {
		return TreeNode{}, fmt.Errorf("invalid hash")
	}

	return TreeNode{Value: convertedVal}, nil
}

func getChildrenOf(i int, nodes []TreeNode) (TreeNode, TreeNode) {
	return nodes[i*2+1], nodes[i*2+2]
}
