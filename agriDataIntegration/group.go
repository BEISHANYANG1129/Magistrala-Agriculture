package agridataintegration

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Group 表示云平台的分组信息
type Group struct {
	GroupId   string `json:"groupId"`   // 分组 ID
	GroupName string `json:"groupName"` // 分组名
	ParentId  string `json:"parentId"`  // 上级组名
}

// GetGroupList 查询分组列表
// 返回：分组信息列表
func (s *PlatformService) GetGroupList() ([]Group, error) {
	api := "/api/device/getGroupList"
	u := s.BaseURL + api

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req)

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Result[[]Group]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != 1000 {
		return nil, fmt.Errorf("GetGroupList failed: %s", result.Message)
	}
	return result.Data, nil
}
