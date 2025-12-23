package services

import (
	"container/heap"
	"math"
	"sion-backend/models"
)

// PathFinder - A* 경로 계획 서비스
type PathFinder struct {
	gridWidth  int
	gridHeight int
	cellSize   float64
	obstacles  []models.Obstacle
}

// Node - A* 노드
type Node struct {
	x, y       int
	g, h, f    float64
	parent     *Node
	index      int // for heap
	worldX     float64
	worldY     float64
}

// PriorityQueue - A* 우선순위 큐
type PriorityQueue []*Node

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].f < pq[j].f
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	node := x.(*Node)
	node.index = n
	*pq = append(*pq, node)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	node.index = -1
	*pq = old[0 : n-1]
	return node
}

// NewPathFinder - PathFinder 생성
func NewPathFinder(width, height int, cellSize float64, obstacles []models.Obstacle) *PathFinder {
	return &PathFinder{
		gridWidth:  width,
		gridHeight: height,
		cellSize:   cellSize,
		obstacles:  obstacles,
	}
}

// FindPath - A* 알고리즘으로 경로 찾기
func (pf *PathFinder) FindPath(start, goal models.PositionData) ([]models.PositionData, bool) {
	// 월드 좌표 → 그리드 좌표
	startNode := pf.worldToGrid(start.X, start.Y)
	goalNode := pf.worldToGrid(goal.X, goal.Y)

	// 시작/목표 유효성 검사
	if !pf.isValid(startNode.x, startNode.y) || !pf.isValid(goalNode.x, goalNode.y) {
		return nil, false
	}

	// 장애물 체크
	if pf.isObstacle(startNode.x, startNode.y) || pf.isObstacle(goalNode.x, goalNode.y) {
		return nil, false
	}

	// A* 초기화
	openSet := make(PriorityQueue, 0)
	heap.Init(&openSet)
	closedSet := make(map[string]bool)

	startNode.g = 0
	startNode.h = pf.heuristic(startNode.x, startNode.y, goalNode.x, goalNode.y)
	startNode.f = startNode.g + startNode.h
	heap.Push(&openSet, startNode)

	// A* 메인 루프
	for openSet.Len() > 0 {
		current := heap.Pop(&openSet).(*Node)

		// 목표 도달
		if current.x == goalNode.x && current.y == goalNode.y {
			return pf.reconstructPath(current), true
		}

		key := nodeKey(current.x, current.y)
		closedSet[key] = true

		// 이웃 노드 탐색 (8방향)
		for _, dir := range pf.getDirections() {
			nx, ny := current.x+dir[0], current.y+dir[1]

			if !pf.isValid(nx, ny) || pf.isObstacle(nx, ny) {
				continue
			}

			neighborKey := nodeKey(nx, ny)
			if closedSet[neighborKey] {
				continue
			}

			// 이동 비용 계산 (대각선은 √2)
			moveCost := 1.0
			if dir[0] != 0 && dir[1] != 0 {
				moveCost = 1.414
			}

			tentativeG := current.g + moveCost

			// 더 나은 경로 발견
			neighbor := pf.worldToGrid(float64(nx)*pf.cellSize, float64(ny)*pf.cellSize)
			neighbor.g = tentativeG
			neighbor.h = pf.heuristic(nx, ny, goalNode.x, goalNode.y)
			neighbor.f = neighbor.g + neighbor.h
			neighbor.parent = current

			heap.Push(&openSet, neighbor)
		}
	}

	// 경로 없음
	return nil, false
}

// worldToGrid - 월드 좌표 → 그리드 좌표
func (pf *PathFinder) worldToGrid(x, y float64) *Node {
	gx := int(x / pf.cellSize)
	gy := int(y / pf.cellSize)
	return &Node{
		x:      gx,
		y:      gy,
		worldX: x,
		worldY: y,
	}
}

// isValid - 그리드 범위 내 검사
func (pf *PathFinder) isValid(x, y int) bool {
	return x >= 0 && x < pf.gridWidth && y >= 0 && y < pf.gridHeight
}

// isObstacle - 장애물 충돌 검사
func (pf *PathFinder) isObstacle(x, y int) bool {
	worldX := float64(x) * pf.cellSize
	worldY := float64(y) * pf.cellSize

	for _, obs := range pf.obstacles {
		dx := worldX - obs.Position.X
		dy := worldY - obs.Position.Y
		dist := math.Sqrt(dx*dx + dy*dy)

		// 장애물 반경 + 안전 마진 (0.3m)
		if dist < obs.Radius+0.3 {
			return true
		}
	}
	return false
}

// heuristic - 휴리스틱 함수 (유클리드 거리)
func (pf *PathFinder) heuristic(x1, y1, x2, y2 int) float64 {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	return math.Sqrt(dx*dx + dy*dy)
}

// getDirections - 8방향 이동
func (pf *PathFinder) getDirections() [][2]int {
	return [][2]int{
		{0, 1}, {1, 0}, {0, -1}, {-1, 0}, // 상하좌우
		{1, 1}, {1, -1}, {-1, 1}, {-1, -1}, // 대각선
	}
}

// reconstructPath - 경로 재구성
func (pf *PathFinder) reconstructPath(node *Node) []models.PositionData {
	path := []models.PositionData{}
	current := node

	for current != nil {
		path = append([]models.PositionData{
			{
				X: float64(current.x) * pf.cellSize,
				Y: float64(current.y) * pf.cellSize,
			},
		}, path...)
		current = current.parent
	}

	// 경로 간소화 (Douglas-Peucker)
	return pf.simplifyPath(path, 0.5)
}

// simplifyPath - Douglas-Peucker 알고리즘으로 경로 간소화
func (pf *PathFinder) simplifyPath(path []models.PositionData, epsilon float64) []models.PositionData {
	if len(path) < 3 {
		return path
	}

	// 가장 먼 점 찾기
	dmax := 0.0
	index := 0
	for i := 1; i < len(path)-1; i++ {
		d := pf.perpendicularDistance(path[i], path[0], path[len(path)-1])
		if d > dmax {
			index = i
			dmax = d
		}
	}

	// 재귀적으로 간소화
	if dmax > epsilon {
		left := pf.simplifyPath(path[:index+1], epsilon)
		right := pf.simplifyPath(path[index:], epsilon)
		return append(left[:len(left)-1], right...)
	}

	return []models.PositionData{path[0], path[len(path)-1]}
}

// perpendicularDistance - 점에서 선분까지 수직 거리
func (pf *PathFinder) perpendicularDistance(point, lineStart, lineEnd models.PositionData) float64 {
	dx := lineEnd.X - lineStart.X
	dy := lineEnd.Y - lineStart.Y

	if dx == 0 && dy == 0 {
		return math.Sqrt(
			math.Pow(point.X-lineStart.X, 2) +
				math.Pow(point.Y-lineStart.Y, 2),
		)
	}

	t := ((point.X-lineStart.X)*dx + (point.Y-lineStart.Y)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))

	projX := lineStart.X + t*dx
	projY := lineStart.Y + t*dy

	return math.Sqrt(
		math.Pow(point.X-projX, 2) +
			math.Pow(point.Y-projY, 2),
	)
}

// nodeKey - 노드 키 생성
func nodeKey(x, y int) string {
	return string(rune(x))<<16 | string(rune(y))
}
