import { useEffect, useState } from 'react';
import SubCard from 'ui-component/cards/SubCard';
import { Alert, Box, Button, Checkbox, FormControlLabel, Stack, Typography } from '@mui/material';
import Grid from '@mui/material/Unstable_Grid2';
import { showError, showSuccess } from 'utils/common';
import { API } from 'utils/api';
import { allChannelTypeOptions, CHANNEL_TYPE_OPTION_KEY, invalidateChannelTypeOptions, parseChannelTypeOptionValue } from 'utils/channelTypeOptions';

const channelTypeOptions = allChannelTypeOptions();

const ChannelSetting = () => {
  const [selected, setSelected] = useState([]);
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      const option = data.find((item) => item.key === CHANNEL_TYPE_OPTION_KEY);
      setSelected(option ? parseChannelTypeOptionValue(option.value) : []);
      setLoaded(true);
    } else {
      showError(message);
    }
  };

  useEffect(() => {
    getOptions().then();
  }, []);

  const handleToggle = (value) => {
    setSelected((selected) => (selected.includes(value) ? selected.filter((item) => item !== value) : [...selected, value]));
  };

  const submitConfig = async () => {
    setLoading(true);
    const value = selected.join(',');
    const res = await API.put('/api/option/', {
      key: CHANNEL_TYPE_OPTION_KEY,
      value
    });
    const { success, message } = res.data;
    if (success) {
      invalidateChannelTypeOptions();
      showSuccess('保存成功');
    } else {
      showError(message);
    }
    setLoading(false);
  };

  return (
    <Stack spacing={2}>
      <SubCard title="渠道设置">
        <Stack spacing={2}>
          <Alert severity="info">
            勾选后，只有被勾选的渠道类型才会出现在「新建渠道」和「新建模型」的渠道下拉列表中。一个都不勾选则显示全部渠道类型。
          </Alert>
          <Stack direction="row" spacing={2} alignItems="center" flexWrap="wrap">
            <Typography variant="body2">
              已勾选 {selected.length} / {channelTypeOptions.length}
            </Typography>
            <Button size="small" onClick={() => setSelected(channelTypeOptions.map((option) => option.value))} disabled={loading || !loaded}>
              全选
            </Button>
            <Button size="small" onClick={() => setSelected([])} disabled={loading || !loaded}>
              取消全选
            </Button>
          </Stack>
          <Box>
            <Grid container spacing={{ xs: 1, sm: 2 }}>
              {channelTypeOptions.map((option) => (
                <Grid xs={12} sm={6} md={4} lg={3} key={option.value}>
                  <FormControlLabel
                    control={
                      <Checkbox
                        size="small"
                        checked={selected.includes(option.value)}
                        onChange={() => handleToggle(option.value)}
                        disabled={loading || !loaded}
                      />
                    }
                    label={option.text}
                  />
                </Grid>
              ))}
            </Grid>
          </Box>
          <Box>
            <Button variant="contained" onClick={submitConfig} disabled={loading || !loaded}>
              保存渠道设置
            </Button>
          </Box>
        </Stack>
      </SubCard>
    </Stack>
  );
};

export default ChannelSetting;
