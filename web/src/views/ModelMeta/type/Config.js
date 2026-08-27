const defaultConfig = {
  input: {
    channel_type: 1,
    model: '',
    status: 1,
    input_price: 0.0,
    output_price: 0.0,
    cache_price: 0.0,
    price_unit: 'CNY'
  },
  inputLabel: {
    channel_type: '渠道类型',
    model: '模型名称',
    status: '启用',
    input_price: '输入价格（每百万 token）',
    output_price: '输出价格（每百万 token）',
    cache_price: '缓存价格（每百万 token）',
    price_unit: '价格单位'
  },
  prompt: {
    channel_type: '请选择渠道类型',
    model: '请添加模型名称',
    status: '是否启用',
    input_price: '例如：0.12，CNY 单位即 ¥0.12 / 百万 token',
    output_price: '例如：0.36，CNY 单位即 ¥0.36 / 百万 token',
    cache_price: '0 = 未配置（缓存 token 按输入价格收费）；> 0 才启用缓存折扣，例：0.06',
    price_unit: '选择价格货币单位'
  }
};


export { defaultConfig };
