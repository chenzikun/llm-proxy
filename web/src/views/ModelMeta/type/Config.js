const BILLING_UNITS = [
  { value: 'token', label: 'token（文本 / 按 token 计价的图片模型）', priceUnit: '百万 token' },
  { value: 'char', label: 'char 字符（TTS 语音合成）', priceUnit: '百万字符' },
  { value: 'second', label: 'second 秒（语音转写 / 翻译）', priceUnit: '百万秒' },
  { value: 'image', label: 'image 张（按张计价的图片模型）', priceUnit: '张' }
];

const defaultConfig = {
  input: {
    channel_type: 1,
    model: '',
    status: 1,
    input_price: 0.0,
    output_price: 0.0,
    cache_price: 0.0,
    price_unit: 'CNY',
    billing_unit: 'token'
  },
  inputLabel: {
    channel_type: '渠道类型',
    model: '模型名称',
    status: '启用',
    input_price: '输入价格',
    output_price: '输出价格',
    cache_price: '缓存价格',
    price_unit: '价格单位',
    billing_unit: '计量单位'
  },
  prompt: {
    channel_type: '请选择渠道类型',
    model: '请添加模型名称',
    status: '是否启用',
    input_price: '',
    output_price: '',
    cache_price: '0 = 未配置（缓存 token 按输入价格收费）；> 0 才启用缓存折扣',
    price_unit: '选择价格货币单位',
    billing_unit: '决定价格字段中“每百万”的单位是什么'
  }
};

export { defaultConfig, BILLING_UNITS };
