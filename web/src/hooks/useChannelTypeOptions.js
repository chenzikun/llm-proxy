import { useEffect, useState } from 'react';
import { allChannelTypeOptions, loadChannelTypeOptions } from 'utils/channelTypeOptions';

// 返回「渠道设置」里勾选的渠道类型；未勾选时返回全部
const useChannelTypeOptions = () => {
  const [options, setOptions] = useState(allChannelTypeOptions);

  useEffect(() => {
    let active = true;
    loadChannelTypeOptions().then((channelTypes) => {
      if (active) {
        setOptions(channelTypes);
      }
    });
    return () => {
      active = false;
    };
  }, []);

  return options;
};

export default useChannelTypeOptions;
