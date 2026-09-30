import axios from 'axios';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import { API } from 'utils/api';

// 设置页「渠道设置」写入的 option key，值为逗号分隔的渠道类型 ID
export const CHANNEL_TYPE_OPTION_KEY = 'ChannelTypeOptions';

let enabledCache = null;
let loadingPromise = null;

export const allChannelTypeOptions = () => Object.values(CHANNEL_OPTIONS).sort((a, b) => a.text.localeCompare(b.text));

export const parseChannelTypeOptionValue = (value) =>
  String(value || '')
    .split(',')
    .map((item) => parseInt(item.trim(), 10))
    .filter((item) => !Number.isNaN(item));

// 读取失败时不写缓存，下次进弹窗还会重试
export const loadEnabledChannelTypes = () => {
  if (enabledCache) {
    return Promise.resolve(enabledCache);
  }
  if (!loadingPromise) {
    // /api/option 仅 root 可读，这里绕过 API 的全局错误提示：
    // 非 root 管理员读不到时静默降级为「显示全部」，不该在渠道/模型页面弹错误
    loadingPromise = axios
      .get('/api/option/', { baseURL: API.defaults.baseURL })
      .then((res) => {
        const { success, data } = res.data;
        const option = success && Array.isArray(data) && data.find((item) => item.key === CHANNEL_TYPE_OPTION_KEY);
        enabledCache = option ? parseChannelTypeOptionValue(option.value) : [];
        return enabledCache;
      })
      .catch(() => [])
      .finally(() => {
        loadingPromise = null;
      });
  }
  return loadingPromise;
};

// 未勾选任何渠道类型时不过滤，保持改动前的行为
export const filterChannelTypeOptions = (options, enabledTypes) => {
  if (!enabledTypes || enabledTypes.length === 0) {
    return options;
  }
  return options.filter((option) => enabledTypes.includes(option.value));
};

export const loadChannelTypeOptions = () =>
  loadEnabledChannelTypes().then((enabledTypes) => filterChannelTypeOptions(allChannelTypeOptions(), enabledTypes));

// 新建时的默认类型：默认类型没被勾选时，落到第一个已勾选类型
export const resolveNewChannelType = (options, defaultType) => {
  if (!options || options.length === 0) {
    return defaultType;
  }
  return options.some((option) => option.value === defaultType) ? defaultType : options[0].value;
};

// 编辑已有记录时，当前值即使未被勾选也要保留，否则下拉框会显示为空
export const withCurrentChannelType = (options, currentValue, isEdit) => {
  if (!isEdit) {
    return options;
  }
  const value = Number(currentValue);
  if (!value || options.some((option) => option.value === value)) {
    return options;
  }
  const current = CHANNEL_OPTIONS[value];
  return current ? [...options, current] : options;
};

export const invalidateChannelTypeOptions = () => {
  enabledCache = null;
};
