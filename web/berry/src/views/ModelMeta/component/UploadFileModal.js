import PropTypes from 'prop-types';
import { useState } from 'react';
import { useTheme } from '@mui/material/styles';
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
  Box,
  LinearProgress,
  Typography
} from '@mui/material';

import { Formik } from 'formik';
import * as Yup from 'yup';

// 文件上传验证模式
const validationSchema = Yup.object().shape({
  file: Yup.mixed()
    .required('请选择文件')
    .test('fileFormat', '仅支持JSON或CSV文件', (value) => {
      if (!value) return false;
      return ['application/json', 'text/csv'].includes(value.type) ||
             value.name.endsWith('.json') ||
             value.name.endsWith('.csv');
    })
});

const UploadFileModal = ({ open, onCancel, onOk }) => {
  const theme = useTheme();
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);

  const handleFileUpload = async (values, { setErrors, setStatus, setSubmitting }) => {
    setSubmitting(true);
    setUploading(true);
    setUploadProgress(0);

    try {
      const formData = new FormData();
      formData.append('file', values.file);

      const res = await API.post('/api/model-meta/upload', formData, {
        headers: {
          'Content-Type': 'multipart/form-data'
        },
        onUploadProgress: (progressEvent) => {
          const percentCompleted = Math.round((progressEvent.loaded * 100) / progressEvent.total);
          setUploadProgress(percentCompleted);
        }
      });

      const { success, message, data } = res.data;
      if (success) {
        showSuccess('文件上传成功！');
        setStatus({ success: true });
        onOk(true);
      } else {
        setStatus({ success: false });
        showError(message);
        setErrors({ submit: message });
      }
    } catch (error) {
      setStatus({ success: false });
      showError(error.message || '上传失败');
      setErrors({ submit: error.message || '上传失败' });
    } finally {
      setSubmitting(false);
      setUploading(false);
    }
  };

  return (
    <Dialog open={open} onClose={onCancel} fullWidth maxWidth={'sm'}>
      <DialogTitle
        sx={{
          margin: '0px',
          fontWeight: 700,
          lineHeight: '1.55556',
          padding: '24px',
          fontSize: '1.125rem'
        }}
      >
        上传模型文件
      </DialogTitle>
      <Divider />
      <DialogContent>
        <Formik
          initialValues={{ file: null }}
          validationSchema={validationSchema}
          onSubmit={handleFileUpload}
        >
          {({ errors, handleSubmit, setFieldValue, touched, values, isSubmitting }) => (
            <form noValidate onSubmit={handleSubmit}>
              <FormControl fullWidth error={Boolean(touched.file && errors.file)} sx={{ ...theme.typography.otherInput }}>
                <Box
                  sx={{
                    border: '1px dashed',
                    borderColor: touched.file && errors.file ? 'error.main' : 'grey.400',
                    borderRadius: 1,
                    p: 2,
                    textAlign: 'center',
                    cursor: 'pointer',
                    '&:hover': {
                      borderColor: 'primary.main'
                    }
                  }}
                  onClick={() => document.getElementById('file-upload').click()}
                >
                  <input
                    id="file-upload"
                    type="file"
                    accept=".json,.csv"
                    style={{ display: 'none' }}
                    onChange={(event) => {
                      setFieldValue('file', event.currentTarget.files[0]);
                    }}
                  />
                  <Typography variant="body1" color="textSecondary">
                    {values.file ? values.file.name : '点击选择文件或拖拽文件到此处'}
                  </Typography>
                  <Typography variant="caption" color="textSecondary">
                    支持的文件格式：JSON, CSV
                  </Typography>
                </Box>
                {touched.file && errors.file && (
                  <FormHelperText error id="helper-text-file-upload">
                    {errors.file}
                  </FormHelperText>
                )}
              </FormControl>

              {uploading && (
                <Box sx={{ width: '100%', mt: 2 }}>
                  <LinearProgress variant="determinate" value={uploadProgress} />
                  <Typography variant="caption" align="center" display="block">
                    上传进度: {uploadProgress}%
                  </Typography>
                </Box>
              )}

              <DialogActions>
                <Button onClick={onCancel}>取消</Button>
                <Button
                  disableElevation
                  disabled={isSubmitting || !values.file}
                  type="submit"
                  variant="contained"
                  color="primary"
                >
                  上传
                </Button>
              </DialogActions>
            </form>
          )}
        </Formik>
      </DialogContent>
    </Dialog>
  );
};

UploadFileModal.propTypes = {
  open: PropTypes.bool,
  onCancel: PropTypes.func,
  onOk: PropTypes.func
};

export default UploadFileModal;
