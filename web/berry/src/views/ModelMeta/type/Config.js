const defaultConfig = {
  input: {
    channel_type: 1,
    model: '',
    status: 1,
    model_ratio: 0.0,
    completion_ratio: 0.0
  },
  inputLabel: {
    channel_type: '渠道类型',
    model: '模型名称',
    status: '启用',
    model_ratio: '模型费率',
    completion_ratio: '模型补全费率'
  },
  prompt: {
    channel_type: '请选择渠道类型',
    model: '请添加模型名称',
    status: '是否启用',
    model_ratio: '请输入模型费率(百万Token)',
    completion_ratio: '请输入模型补全费率(百万Token)'
  }
};


export { defaultConfig };
