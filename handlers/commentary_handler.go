package handlers

import (
	"sion-backend/services"
)

// CommentarySvc - 자동 중계 서비스 (main.go에서 초기화)
var CommentarySvc *services.CommentaryService

// AGVMgr - AGV 관리자 (main.go에서 초기화)
var AGVMgr *AGVManager

// TriggerCommentary - 외부에서 해설 이벤트 트리거
func TriggerCommentary(eventType string, data map[string]interface{}) {
	if CommentarySvc != nil {
		CommentarySvc.QueueEvent(eventType, data)
	}
}

// SetCommentaryEnabled - 자동 중계 활성화/비활성화
func SetCommentaryEnabled(enabled bool) {
	if CommentarySvc != nil {
		CommentarySvc.SetEnabled(enabled)
	}
}
