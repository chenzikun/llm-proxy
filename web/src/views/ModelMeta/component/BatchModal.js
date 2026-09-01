import PropTypes from 'prop-types';
import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Divider,
  FormControl,
  FormHelperText,
  TextField,
  Box,
  Typography,
} from '@mui/material';

import { Formik } from 'formik';
import * as Yup from 'yup';

// 验证模式
const validationSchema = Yup.object().shape({
  content: Yup.string().required('内容不能为空')
});

const BatchModal = ({ open, onCancel, onOk }) => {
  const handleBatchSubmit = async (values, { setErrors, setStatus, setSubmitting }) => {
    setSubmitting(true);
    try {
      // 检查是否以model开始
      if (!values.content.trim().startsWith('model')) {
        // 自动添加header行
        values.content =
          'model|channel_type|input_price|output_price|cache_price|price_unit|billing_unit\n' + values.content;
      }

      // 发送批量请求
      const res = await API.post('/api/model-meta/batch_add', { text: values.content });

      const { success, message } = res.data;
      if (success) {
        showSuccess('批量添加模型成功！');
        setStatus({ success: true });
        onOk(true);
      } else {
        setStatus({ success: false });
        showError(message || '添加失败');
        setErrors({ submit: message || '添加失败' });
      }
    } catch (error) {
      setStatus({ success: false });
      showError(error.message || '批量添加失败');
      setErrors({ submit: error.message || '批量添加失败' });
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onClose={onCancel} fullWidth maxWidth={'md'}>
      <DialogTitle
        sx={{
          margin: '0px',
          fontWeight: 700,
          lineHeight: '1.55556',
          padding: '24px',
          fontSize: '1.125rem'
        }}
      >
        批量添加模型
      </DialogTitle>
      <Divider />
      <DialogContent>
        <Formik
          initialValues={{ content: '' }}
          validationSchema={validationSchema}
          onSubmit={handleBatchSubmit}
        >
          {({ errors, handleBlur, handleChange, handleSubmit, isSubmitting, touched, values }) => (
            <form noValidate onSubmit={handleSubmit}>
              <Box sx={{ mt: 2, mb: 1 }}>
                <Typography variant="subtitle2" color="textSecondary">
                  格式说明：每行一个模型，以竖线 | 分隔，字段顺序：模型名称 | 渠道类型 | 输入价格 | 输出价格 | 缓存价格 | 价格单位 | 计量单位
                </Typography>
                <Typography variant="caption" color="textSecondary">
                  价格单位：CNY（人民币）或 USD（美元）；缓存价格填 0 表示未配置
                </Typography>
                <Typography variant="caption" color="textSecondary" display="block" sx={{ mt: 0.5 }}>
                  计量单位：token（每百万 token）/ char（每百万字符，TTS）/ second（每百万秒，语音转写）/ image（每百万张）；留空按 token 处理
                </Typography>
                <Typography variant="caption" color="textSecondary" display="block" sx={{ mt: 0.5 }}>
                  例如：claude-3-5-sonnet|37|3|15|1.5|USD|token &nbsp;&nbsp; 或 &nbsp;&nbsp; my-tts|37|0|50|0|CNY|char
                </Typography>
              </Box>

              <FormControl fullWidth error={Boolean(touched.content && errors.content)} sx={{ mt: 2 }}>
                <TextField
                  id="content"
                  label="批量内容"
                  multiline
                  rows={10}
                  value={values.content}
                  name="content"
                  onBlur={handleBlur}
                  onChange={handleChange}
                  error={Boolean(touched.content && errors.content)}
                  placeholder="模型名称|渠道类型|输入价格|输出价格|缓存价格|价格单位|计量单位"
                />
                {touched.content && errors.content && (
                  <FormHelperText error id="helper-text-content">
                    {errors.content}
                  </FormHelperText>
                )}
              </FormControl>

              <DialogActions>
                <Button onClick={onCancel}>取消</Button>
                <Button
                  disableElevation
                  disabled={isSubmitting}
                  type="submit"
                  variant="contained"
                  color="primary"
                >
                  提交
                </Button>
              </DialogActions>
            </form>
          )}
        </Formik>
      </DialogContent>
    </Dialog>
  );
};

BatchModal.propTypes = {
  open: PropTypes.bool,
  onCancel: PropTypes.func,
  onOk: PropTypes.func
};

export default BatchModal;
