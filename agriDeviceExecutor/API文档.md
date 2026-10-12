### 八、智慧环控2.0 设备和智慧环控通用3.0 设备
#### 8.1 批量获取设备详情
| 接口编号 | 8.1 |
| --- | --- |
| 接口名称 | 批量获取设备详情 |
| 功能描述 | 批量获取设备详情 |
| 接口地址 | /api/v2.0/irrigation/node/getDeviceIii |
| 请求方式 | GET |
| 参数格式 |  |
| 返回数据格式 | JSON |
| 备注 |  |


##### 8.1.2 请求参数说明
###### 8.1.2.1 请求头Header 参数
| header | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| token | 是 | string | token |


###### 8.1.2.2 请求参数
| 字段 | 必须 | 类型 | 说明 |
| --- | --- | --- | --- |
| devAddr | 是 | String | 设备地址(多个用英文逗号分隔,最多同时获取5个设备信息) |


##### 8.1.3 返回数据说明
| 参数名 | 类型 | 说明 |
| --- | --- | --- |
| deviceAddr | String | 设备地址 |
| deviceType | 废弃 |  |
| deviceName | String | 设备名称 |
| devicelng | double | 经度 |
| devicelat | double | 纬度 |
| saveDateInterval | int | 数据保存间隔 |
| offlineInterval | int | 离线判断间隔 |
| city | 废弃 |  |
| createTime | long | 创建时间(13 位时间戳) |
| alertDataStatus | String | 报警数据状态0 关闭1 开启 |
| phoneOfflineNotification | int | 手机离线通知状态0 关闭1 开启 |
| phoneAlarmInterval | int | 手机报警间隔(分钟) |
| phoneMaxSendingNumber | int | 手机最大发送次数 |
| emailOfflineNotification | int | 邮件离线通知状态0 关闭1 开启 |
| emailAlarmInterval | int | 邮件报警间隔(分钟) |
| emailMaxSendingNumber | int | 邮件最大发送次数 |
| phone | -- | 废弃 |
| email | -- | 废弃 |
| deviceEnabled | -- | 废弃 |
| deviceIccId | -- | 废弃 |
| groupId | -- | 废弃 |
| authority | -- | 废弃 |
| listIrrigationFactor | -- | 废弃 |
| listTbIrrigationContact |  | 报警通知联系人 |
| deviceAddr | String | 设备地址 |
| contactType | String | 1 手机 2 邮箱 |
| contact | String | 联系方式 |
| updateTime | long | 更新时间( 13 位时间戳) |
| id | int | 设备通信号码 id |
| irrigationNodeDOList |  | 设备节点信息 |
| nodeId | Integer | 节点编号 |
| deviceAddr | String | 设备地址 |
| deviceName | -- | 废弃 |
| factorId | String | 节点 id |
| nodeName | String | 节点名称 |
| enable | Integer | 节点是否开启 0 :关闭 1 :开启 |
| factorType | Integer | 1 采集器 2 阀门 |
| nodeMold | -- | 废弃 |
| nodeType | Integer | 1: 模拟量 1 使能模拟量 2 使能；2: 模拟量 1 使能模拟量 2 禁用；4: 浮点型设备 5: 开关量型设备 6: 32 位有符号整形 7: 32 位无符号整形 8: 遥调设备 |
| priority | Integer | 节点优先级 |
| digits | Integer | 小数位数 |
| temName | String | 模拟量 1 名称 |
| temUnit | String | 模拟量 1 单位 |
| temRatio | Double | 模拟量 1 系数 |
| temOffset | Double | 模拟量 1 偏差 |
| temUpperLimit | Double | 模拟量 1 上限 |
| temLowerLimit | Double | 模拟量 1 下限 |
| humName | String | 模拟量 2 名称 |
| humUnit | String | 模拟量 2 单位 |
| humRatio | Double | 模拟量 2 系数 |
| humOffset | Double | 模拟量 2 偏差 |
| humUpperLimit | Double | 模拟量 2 上限 |
| humLowerLimit | Double | 模拟量 2 下限 |
| switchOnContent | String | 开关量闭合显示内容 |
| switchOffContent | String | 开关量断开显示内容 |
| switchAlarmType | Integer | 开关量报警类型( 0 不报警 1 闭合报警 2 断开报警) |
| smsEnabled | Integer | 短信告警开关, 0, 关; 1, 开 |
| emailEnabled | Integer | 邮件告警开关, 0, 关; 1, 开 |
| offlineAlarmingSwitch | -- | 废弃 |
| offlineAlarmingAlarmContent | -- | 废弃 |
| excessAlarmingSwitch | Integer | 超限报警开关 0: 关; 1: 开 |
| excessAlarmingAlarmContent | String | 报警内容模板 |
| createTime | String | 创建时间 |
| listFactorRegulating | -- | 废弃 |


##### 8.1.4 返回格式示例
```json
{
"code":1000,
"message":"success",
"data":[
{
"deviceAddr":"66668888",
"deviceType":"irrigation2",
"deviceName":"66668888",
"devicelng":115.95706,
"devicelat":39.062559,
"saveDateInterval":1,
"offlineInterval":11,
"city":"济南",
"createTime":1643243753000,
"alertDataStatus":"0",
"phoneOfflineNotification":0,
"phoneAlarmInterval":10,
"phoneMaxSendingNumber":5,
"emailOfflineNotification":0,
"emailAlarmInterval":1,
"emailMaxSendingNumber":3,
"phone":null,
"email":null,
"deviceEnabled":"1",
"deviceIccId":null,
"groupId":null,
"authority":null,
"irrigationNodeDOList":[
{
"nodeId":1,
"deviceAddr":"66668888",
"deviceName":"66668888",
"factorId":"66668888_1",
"nodeName":"节点1",
"enable":1,
"factorType":1,
"nodeMold":0,
"nodeType":1,
"priority":0,
"digits":1,
"temName":"温度",
"temUnit":"℃",
"temRatio":0.1,
"temOffset":0,
"temUpperLimit":100,
"temLowerLimit":0,
"humName":"湿度",
"humUnit":"%RH",
"humRatio":0.1,
"humOffset":0,
"humUpperLimit":100,
"humLowerLimit":0,
"switchOnContent":null,
"switchOffContent":null,
"switchAlarmType":0,
"smsEnabled":0,
"emailEnabled":0,
"offlineAlarmingSwitch":0,
"offlineAlarmingAlarmContent":"[设备名称]-[节点名称]设备地址:[设备地址],节点离线,系统时间:[系统时间]",
"excessAlarmingSwitch":0,
"excessAlarmingAlarmContent":"[设备名称]-[节点名称]设备地址:[设备地址],[报警值],[报警限值]系统时间:[系统时间]",
"createTime":"2022-09-16 13:28:14",
"listFactorRegulating":null
}
],
"listTbIrrigationContact":[]
}
]
}
```

---

#### 8.2 修改设备信息
| 接口编号 | 8.2 |
| --- | --- |
| 接口名称 | 修改设备信息 |
| 功能描述 | 修改设备信息 |
| 接口地址 | /api/v2.0/irrigation/device/updateDevInfo |
| 请求方式 | POST |
| 参数格式 | JSON |
| 返回数据格式 | JSON |
| 备注 |  |


##### 8.2.2 请求参数说明
###### 8.2.2.1 请求头Header 参数
| header | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| token | 是 | string | token |


###### 8.2.2.2 请求参数
| 字段 | 必须 | 类型 | 说明 |
| --- | --- | --- | --- |
| deviceAddr | 是 | String | 设备地址 |
| deviceName | 否 | string | 设备名称 |
| devicelng | 否 | double | 经度 |
| devicelat | 否 | double | 纬度 |
| saveDateInterval | 否 | integer | 数据保存间隔 |
| offlineInterval | 否 | integer | 离线判断间隔 |
| alertDataStatus | 否 | string | 报警数据状态 0 关闭 1 开启 |
| phoneOfflineNotification | 否 | integer | 手机离线通知状态 0 关闭 1 开启 |
| phoneAlarmInterval | 否 | integer | 手机报警间隔(分钟) |
| phoneMaxSendingNumber | 否 | integer | 手机最大发送次数 |
| emailOfflineNotification | 否 | integer | 邮件离线通知状态 0 关闭 1 开启 |
| emailAlarmInterval | 否 | integer | 邮件报警间隔(分钟) |
| emailMaxSendingNumber | 否 | integer | 邮件最大发送次数 |


##### 8.2.3 返回数据说明
| 参数名 | 类型 | 说明 |
| --- | --- | --- |


##### 8.2.4 返回格式示例
```json
{ "code": 1000, "message": "success", "data": null }
```

---

#### 8.3 获取节点列表
| 接口编号 | 8.3 |
| --- | --- |
| 接口名称 | 获取节点列表 |
| 功能描述 | 获取节点列表 |
| 接口地址 | /api/v2.0/irrigation/node/getDeviceNodeList |
| 请求方式 | GET |
| 参数格式 |  |
| 返回数据格式 | JSON |
| 备注 |  |


##### 8.3.2 请求参数说明
###### 8.3.2.1 请求头Header 参数
| header | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| token | 是 | string | token |


###### 8.3.2.2 请求参数
| 字段 | 必须 | 类型 | 说明 |
| --- | --- | --- | --- |
| devAddr | 是 | String | 设备地址 |


##### 8.3.3 返回数据说明
| 参数名 | 类型 | 说明 |
| --- | --- | --- |
| nodeId | Integer | 节点编号 |
| deviceAddr | String | 设备地址 |
| deviceName | -- | 废弃 |
| factorId | String | 节点 id |
| nodeName | String | 节点名称 |
| enable | Integer | 节点是否开启 0 :关闭 1 :开启 |
| factorType | Integer | 1 采集器 2 阀门 |
| nodeMold | -- | 废弃 |
| nodeType | Integer | 1: 模拟量 1 使能模拟量 2 使能；3: 模拟量 1 禁用模拟量 2 使能；5: 开关量型设备 |
| priority | Integer | 节点优先级 |
| digits | Integer | 小数位数 |
| temName | String | 模拟量 1 名称 |
| temUnit | String | 模拟量 1 单位 |
| temRatio | Double | 模拟量 1 系数 |
| temOffset | Double | 模拟量 1 偏差 |
| temUpperLimit | Double | 模拟量 1 上限 |
| temLowerLimit | Double | 模拟量 1 下限 |
| humName | String | 模拟量 2 名称 |
| humUnit | String | 模拟量 2 单位 |
| humRatio | Double | 模拟量 2 系数 |
| humOffset | Double | 模拟量 2 偏差 |
| humUpperLimit | Double | 模拟量 2 上限 |
| humLowerLimit | Double | 模拟量 2 下限 |
| switchOnContent | String | 开关量闭合显示内容 |
| switchOffContent | String | 开关量断开显示内容 |
| switchAlarmType | Integer | 开关量报警类型( 0 不报警 1 闭合报警 2 断开报警) |
| smsEnabled | Integer | 短信告警开关, 0, 关; 1, 开 |
| emailEnabled | Integer | 邮件告警开关, 0, 关; 1, 开 |
| offlineAlarmingSwitch | -- | 废弃 |
| offlineAlarmingAlarmContent | -- | 废弃 |
| excessAlarmingSwitch | Integer | 超限报警开关 0: 关; 1: 开 |
| excessAlarmingAlarmContent | String | 报警内容模板 |
| createTime | String | 创建时间 |
| listFactorRegulating | -- | 废弃 |


##### 8.3.4 返回格式示例
```json
{
"code":1000,
"message":"成功",
"data":[
{
"nodeId":1,
"deviceAddr":"21104619",
"deviceName":"21104619",
"factorId":"21104619_1",
"nodeName":"节点1",
"enable":1,
"factorType":1,
"nodeMold":0,
"nodeType":1,
"priority":0,
"digits":1,
"temName":"温度",
"temUnit":"℃",
"temRatio":0.1,
"temOffset":0,
"temUpperLimit":100,
"temLowerLimit":0,
"humName":"湿度",
"humUnit":"%RH",
"humRatio":0.1,
"humOffset":0,
"humUpperLimit":100,
"humLowerLimit":0,
"switchOnContent":null,
"switchOffContent":null,
"switchAlarmType":0,
"smsEnabled":0,
"emailEnabled":0,
"offlineAlarmingSwitch":0,
"offlineAlarmingAlarmContent":"[设备名称]-[节点名称]设备地址:[设备地址],节点离线,系统时间:[系统时间]",
"excessAlarmingSwitch":0,
"excessAlarmingAlarmContent":"[设备名称]-[节点名称]设备地址:[设备地址],[报警值],[报警限值]系统时间:[系统时间]",
"createTime":"2022-09-16 13:28:14",
"listFactorRegulating":null
}
]
}
```

---

#### 8.4 修改节点信息
| 接口编号 | 8.4 |
| --- | --- |
| 接口名称 | 修改节点信息 |
| 功能描述 | 修改节点信息 |
| 接口地址 | /api/v2.0/irrigation/node/updateDeviceNode |
| 请求方式 | POST |
| 参数格式 | JSON |
| 返回数据格式 | JSON |
| 备注 |  |


##### 8.4.2 请求参数说明
###### 8.4.2.1 请求头Header 参数
| header | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| token | 是 | string | token |


###### 8.4.2.2 请求参数
| 字段 | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| deviceAddr | 是 | string | 设备地址 |
| nodeId | 是 | integer | 节点编号 |
| nodeType | 否 | integer | 1: 模拟量 1 使能模拟量 2 使能；2: 模拟量 1 使能模拟量 2 禁用；3: 模拟量 1 禁用模拟量 2 使能；4: 浮点型设备；5: 开关量型设备；6: 32 位有符号整形；7: 32 位无符号整形；8: 遥调设备 |
| excessAlarmingSwitch | 否 | integer | 超限报警开关0:关;1:开 |
| digits | 否 | integer | 小数位数 |
| enable | 否 | integer | 节点是否开启0:关闭1:开启 |
| humLowerLimit | 否 | number | 模拟量2 下限 |
| humName | 否 | string | 模拟量2 名称 |
| humOffset | 否 | number | 模拟量2 偏差 |
| humRatio | 否 | number | 模拟量2 系数 |
| humUnit | 否 | string | 模拟量2 单位 |
| humUpperLimit | 否 | number | 模拟量2 上限 |
| nodeName | 否 | string | 节点名称 |
| priority | 否 | integer | 优先级 |
| switchAlarmType | 否 | integer | 开关量报警类型(0 不报警1 闭合报警2 断开报警) |
| switchOffContent | 否 | string | 开关量断开显示内容 |
| switchOnContent | 否 | string | 开关量闭合显示内容 |
| temLowerLimit | 否 | number | 模拟量1 下限 |
| temName | 否 | string | 模拟量1 名称 |
| temOffset | 否 | number | 模拟量1 偏差 |
| temRatio | 否 | number | 模拟量1 系数 |
| temUnit | 否 | string | 模拟量1 单位 |
| temUpperLimit | 否 | number | 模拟量1 上限 |
| smsEnabled | 否 | Integer | 短信告警开关,0,关;1,开 |
| emailEnabled | 否 | Integer | 邮件告警开关,0,关;1,开 |
| excessAlarmingSwitch | 否 | Integer | 超限报警开关0:关;1:开 |
| excessAlarmingAlarmContent | 否 | String | 报警内容模板 |


##### 8.4.3 返回数据说明
| 参数名 | 类型 | 说明 |
| --- | --- | --- |


##### 8.4.4 返回格式示例
```json
{
"code": 1000,
"message": "success",
"data": null
}
```

---

#### 8.5 批量开关节点(使能)
| 接口编号 | 8.5 |
| --- | --- |
| 接口名称 | 批量开关节点(使能) |
| 功能描述 | 批量开关节点(使能) |
| 接口地址 | /api/v2.0/irrigation/node/batchNodeEnable |
| 请求方式 | POST |
| 参数格式 | JSON |
| 返回数据格式 | JSON |
| 备注 |  |


##### 8.5.2 请求参数说明
###### 8.5.2.1 请求头Header 参数
| header | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| token | 是 | string | token |


###### 8.5.2.2 请求参数
| 字段 | 必须 | 类型 | 说明 |
| --- | --- | --- | --- |
| devAddr | 是 | string | 设备地址 |
| enable | 是 | string | 节点使能, 0, 关闭; 1, 打开 |
| factorType | 否 | string | 1 采集器 2 阀门 |


##### 8.5.3 返回数据说明
| 参数名 | 类型 | 说明 |
| --- | --- | --- |


##### 8.5.4 返回格式示例
```json
{
"code": 1000,
"message": "success",
"data": null
}
```

---

#### 8.6 获取节点遥调信息
| 接口编号 | 8.6 |
| --- | --- |
| 接口名称 | 获取节点遥调信息 |
| 功能描述 | 获取节点遥调信息 |
| 接口地址 | /api/v2.0/irrigation/factor/getIrrigationFactorRegulating |
| 请求方式 | GET |
| 参数格式 |  |
| 返回数据格式 | JSON |
| 备注 |  |


##### 8.6.2 请求参数说明
###### 8.6.2.1 请求头Header 参数
| header | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| token | 是 | string | token |


###### 8.6.2.2 请求参数
| 字段 | 必须 | 类型 | 说明 |
| --- | --- | --- | --- |
| factorId | 是 | string | 节点 id |


##### 8.6.3 返回数据说明
| 参数名 | 类型 | 说明 |
| --- | --- | --- |
| factorId | string | 节点 id |
| regularValue | double | 遥调值 |
| regularText | string | 遥调显示值 |
| alarmLevel | int | 报警等级 0 不报警 1 报警 |
| createTime | -- | 废弃 |


##### 8.6.4 返回格式示例
```json
{
"code": 1000,
"message": "success",
"data": [
{
"factorId": "88888888_3",
"regularValue": 0,
"regularText": "断开",
"alarmLevel": 0,
"createTime": null
},
{
"factorId": "88888888_3",
"regularValue": 100,
"regularText": "闭合",
"alarmLevel": 0,
"createTime": null
}
]
}
```

---

#### 8.7 更新节点遥调信息(此接口为删除原有信息重新添加)
| 接口编号 | 8.7 |
| --- | --- |
| 接口名称 | 更新节点遥调信息(此接口为删除原有信息重新添加) |
| 功能描述 | 更新节点遥调信息(此接口为删除原有信息重新添加) |
| 接口地址 | /api/v2.0/irrigation/factor/replaceTbIrrigationFactorRegulating |
| 请求方式 | POST |
| 参数格式 | JSON |
| 返回数据格式 | JSON |
| 备注 |  |


##### 8.7.2 请求参数说明
###### 8.7.2.1 请求头Header 参数
| header | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| token | 是 | string | token |


###### 8.7.2.2 请求参数
| 字段 | 必须 | 类型 | 说明 |
| --- | --- | --- | --- |
| listTbIrrigationFactorRegulating | 是 | array |  |
| factorId | 是 | string | 节点 id |
| regularValue | 是 | integer | 遥调档位值 |
| regularText | 是 | string | 遥调档位内容 |
| alarmLevel | 是 | integer | 报警等级0 不报警1 报警 |


##### 8.7.3 返回数据说明
| 参数名 | 类型 | 说明 |
| --- | --- | --- |


##### 8.7.4 返回格式示例
```json
{ "code": 1000, "message": "success", "data": null }
```

---

#### 8.8 历史记录
| 接口编号 | 8.8 |
| --- | --- |
| 接口名称 | 历史记录 |
| 功能描述 | 历史记录 |
| 接口地址 | /api/v2.0/irrigation/node/getHistoryDataList |
| 请求方式 | GET |
| 参数格式 |  |
| 返回数据格式 | JSON |
| 备注 |  |


##### 8.8.2 请求参数说明
###### 8.8.2.1 请求头Header 参数
| header | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| token | 是 | string | token |


###### 8.8.2.2 请求参数
| 字段 | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| deviceAddr | 是 | string | 设备地址 |
| startTime | 是 | string | 开始时间( yyyy-MM-dd HH:mm:ss ) |
| endTime | 是 | string | 结束时间( yyyy-MM-dd HH:mm:ss ) |
| pages | 是 | integer | 页码 |
| limit | 是 | integer | 每页数量(最大 1000 ) |
| nodeId | 否 | string | 节点编号(多个用英文逗号分隔) |


##### 8.8.3 返回数据说明
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| pages | integer | 页码 |
| limit | integer | 每页数量 |
| totalPages | integer | 总页数 |
| total | integer | 总条数 |
| rows |  | 当前页数据 |
| historyId | integer | 记录表 id |
| deviceAddress | string | 设备地址 |
| nodeId | integer | 节点编号 |
| temStr | string | 温度 |
| humStr | string | 湿度 |
| temValue | Double | 温度 |
| humValue | Double | 湿度 |
| recordTime | long | 创建时间 |
| alarmStatus | integer | 是否是报警数据 0 :正常 1 :报警 |


##### 8.8.4 返回格式示例
```json
{
"code": 1000,
"message": "获取成功",
"data": {
"pages": 1,
"limit": 1,
"totalPages": 60,
"total": 60,
"rows": [
{
"historyId": 1345,
"nodeId": 7,
"deviceAddress": "66668888",
"temStr": "0.0",
"humStr": "0.0",
"temValue": 0,
"humValue": 0,
"recordTime": 1660547896000,
"alarmStatus": 0
}
]
}
}
```

---

#### 8.9 修改阀门工作模式
| 接口编号 | 8.9 |
| --- | --- |
| 接口名称 | 修改阀门工作模式 |
| 功能描述 | 修改阀门工作模式 |
| 接口地址 | /api/v2.0/irrigation/factor/updateFactorMode |
| 请求方式 | POST |
| 参数格式 | JSON |
| 返回数据格式 | JSON |
| 备注 |  |


##### 8.9.2 请求参数说明
###### 8.9.2.1 请求头Header 参数
| header | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| token | 是 | string | token |


###### 8.9.2.2 请求参数
| 字段 | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| factorId | 是 | string | 节点 id |
| mode | 是 | string | 模式, 1 为手动, 2 为自动 |


##### 8.9.3 返回数据说明
| 字段 | 类型 |
| --- | --- |


##### 8.9.4 返回格式示例
```json
{
"code": 1000,
"message": "success",
"data": null
}
```

---

#### 8.10 手动开启关闭阀门
| 接口编号 | 8.10 |
| --- | --- |
| 接口名称 | 手动开启关闭阀门 |
| 功能描述 | 手动开启关闭阀门 |
| 接口地址 | /api/v2.0/irrigation/node/manualControlValve |
| 请求方式 | GET |
| 参数格式 |  |
| 返回数据格式 | JSON |
| 备注 |  |


##### 8.10.2 请求参数说明
###### 8.10.2.1 请求头Header 参数
| header | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| token | 是 | string | token |


###### 8.10.2.2 请求参数
| 字段 | 必选 | 类型 | 说明 |
| --- | --- | --- | --- |
| deviceAddr | 是 | string | 设备地址 |
| factorId | 是 | string | 节点 id |
| mode | 是 | string | 0 关闭 1 开启 |


##### 8.10.3 返回数据说明
| 字段 | 类型 |
| --- | --- |


##### 8.10.4 返回格式示例
```json
{
"code": 1000,
"message": "success",
"data": null
}
```

要不要我帮你将这份提取后的内容生成一个可直接保存的**MD文件**？

