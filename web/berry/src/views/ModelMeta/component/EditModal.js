import PropTypes from 'prop-types';
import { useState, useEffect, useCallback } from 'react';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import { useTheme } from '@mui/material/styles';
import { API } from 'utils/api';
import { showError, showSuccess, getChannelModels } from 'utils/common';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Divider,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  OutlinedInput,
  FormHelperText,
  Switch,
} from '@mui/material';

import { Formik } from 'formik';
import * as Yup from 'yup';
import { defaultConfig } from '../type/Config'; //typeConfig


const floatSchema = Yup.number().required('费率 不能为空').min(0.0000, 'Must be greater than 0').transform((value, originalValue) => {
  // 使用原值进行格式化，比如 '0.0' 不会变成 '0'
  if (typeof originalValue === 'string' && originalValue.includes('.')) {
    return parseFloat(originalValue);
  }
  return value;
});

const validationSchema = Yup.object().shape({
  channel_type: Yup.number().required('渠道 不能为空'),
  model: Yup.string().required('名称 不能为空'),
  status: Yup.number(),
  model_ratio: floatSchema,
  completion_ratio: floatSchema
});

const EditModal = ({ open, modelMetaId, onCancel, onOk }) => {
  const theme = useTheme();
  // const [loading, setLoading] = useState(false);
  const [initialInput, setInitialInput] = useState(defaultConfig.input);
  const [inputLabel, setInputLabel] = useState(defaultConfig.inputLabel); //
  const [inputPrompt, setInputPrompt] = useState(defaultConfig.prompt);

  // const [modelOptions, setModelOptions] = useState([]);
  // const [batchAdd, setBatchAdd] = useState(false);
  const [basicModels, setBasicModels] = useState([]);

  const initModelMeta = () => {
    setInputLabel(defaultConfig.inputLabel);
    setInputPrompt(defaultConfig.prompt);
    return defaultConfig.input;
  };

  // 保留，校验channel type
  const handleTypeChange = (setFieldValue, ChannelId, values) => {
    let localModels = getChannelModels(ChannelId);
    setBasicModels(localModels);
  };

  const submit = async (values, { setErrors, setStatus, setSubmitting }) => {
    setSubmitting(true);

    let res;
    values.model_ratio = values.model_ratio * 1.0;
    values.completion_ratio = values.completion_ratio * 1.0;
    if (modelMetaId) {
      // 修改时提交
      res = await API.put(`/api/model-meta/`, {
        ...values,
        id: parseInt(modelMetaId)
      });
    } else {
      // 新增时提交
      res = await API.post(`/api/model-meta/`, { ...values });
    }

    const { success, message } = res.data;
    if (success) {
      if (modelMetaId) {
        showSuccess('模型更新成功！');
      } else {
        showSuccess('模型创建成功！');
      }
      setSubmitting(false);
      setStatus({ success: true });
      onOk(true);
    } else {
      setStatus({ success: false });
      showError(message);
      setErrors({ submit: message });
    }
  };

  useEffect(() => {
    const loadModelMeta = async () => {
      let res = await API.get(`/api/model-meta/${modelMetaId}`);
      const { success, message, data } = res.data;
      if (success) {
        initModelMeta();
        setInitialInput(data);
      } else {
        showError(message);
      }
    };

    if (modelMetaId) {
      loadModelMeta().then();
    } else {
      initModelMeta();
      setInitialInput({ ...defaultConfig.input });
    }
  }, [modelMetaId]);

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
        {modelMetaId ? '编辑模型' : '新建模型'}
      </DialogTitle>
      <Divider />
      <DialogContent>
        <Formik initialValues={initialInput} enableReinitialize validationSchema={validationSchema} onSubmit={submit}>
          {({ errors, handleBlur, handleChange, handleSubmit, isSubmitting, touched, values, setFieldValue }) => (
            // 获取 channel 列表
            <form noValidate onSubmit={handleSubmit}>
              {/*选择渠道*/}
              <FormControl fullWidth error={Boolean(touched.channel_type && errors.channel_type)}
                           sx={{ ...theme.typography.otherInput }}>
                <InputLabel htmlFor="channel-channel-id-label">{inputLabel.channel_type}</InputLabel>
                <Select
                  id="channel-channel-id-label"
                  label={inputLabel.channel_type}
                  value={values.channel_type}
                  name="channel_type"
                  onBlur={handleBlur}
                  onChange={(e) => {
                    handleChange(e);
                    handleTypeChange(setFieldValue, e.target.value, values);
                  }}
                  MenuProps={{
                    PaperProps: {
                      style: {
                        maxHeight: 200
                      }
                    }
                  }}
                >
                  {Object.values(CHANNEL_OPTIONS)
                    .sort((a, b) => {
                      return a.text.localeCompare(b.text);
                    })
                    .map((option) => {
                      return (
                        <MenuItem key={option.value} value={option.value}>
                          {option.text}
                        </MenuItem>
                      );
                    })}
                </Select>
                {touched.channel_type && errors.channel_type ? (
                  <FormHelperText error id="helper-tex-channel-type-label">
                    {errors.channel_type}
                  </FormHelperText>
                ) : (
                  <FormHelperText id="helper-tex-channel-type-label"> {inputPrompt.channel_type} </FormHelperText>
                )}
              </FormControl>

              {/*模型名称*/}
              <FormControl fullWidth error={Boolean(touched.model && errors.model)}
                           sx={{ ...theme.typography.otherInput }}>
                <InputLabel htmlFor="channel-name-label">{inputLabel.model}</InputLabel>
                <OutlinedInput
                  id="channel-name-label"
                  label={inputLabel.model}
                  type="text"
                  value={values.model}
                  name="model"
                  onBlur={handleBlur}
                  onChange={handleChange}
                  inputProps={{ autoComplete: 'model' }}
                  aria-describedby="helper-text-channel-name-label"
                />
                {touched.model && errors.model ? (
                  <FormHelperText error id="helper-tex-channel-name-label">
                    {errors.model}
                  </FormHelperText>
                ) : (
                  <FormHelperText id="helper-tex-channel-name-label"> {inputPrompt.model} </FormHelperText>
                )}
              </FormControl>

              {/*todo 状态*/}
              <Switch
                checked={values.status === 1}
                onChange={(event) => {
                  console.log(event);
                  const status = event.target.checked ? 1 : 2;
                  setFieldValue('status', status);
                }}
              /> {inputPrompt.status}

              {/*模型费率*/}
              <FormControl fullWidth error={Boolean(touched.model_ratio && errors.model_ratio)}
                           sx={{ ...theme.typography.otherInput }}>
                <InputLabel htmlFor="model-ratio-label">{inputLabel.model_ratio}</InputLabel>
                <OutlinedInput
                  id="model-ratio-label"
                  label={inputLabel.model_ratio}
                  type="number"
                  value={values.model_ratio}
                  name="model_ratio"
                  onBlur={handleBlur}
                  onChange={handleChange}
                  aria-describedby="helper-text-model-ratio-label"
                />

                {touched.model_ratio && errors.model_ratio ? (
                  <FormHelperText error id="helper-tex-model-ratio-label">
                    {errors.model_ratio}
                  </FormHelperText>
                ) : (
                  <FormHelperText id="helper-tex-model-ratio-label"> {inputPrompt.model_ratio} </FormHelperText>
                )}
              </FormControl>

              {/*补全费率*/}
              <FormControl fullWidth error={Boolean(touched.completion_ratio && errors.completion_ratio)}
                           sx={{ ...theme.typography.otherInput }}>
                <InputLabel htmlFor="completion-ratio-label">{inputLabel.completion_ratio}</InputLabel>
                <OutlinedInput
                  id="completion-ratio-label"
                  label={inputLabel.completion_ratio}
                  type="number"
                  value={values.completion_ratio}
                  name="completion_ratio"
                  onBlur={handleBlur}
                  onChange={handleChange}
                  aria-describedby="helper-text-completion-ratio-label"
                />

                {touched.completion_ratio && errors.completion_ratio ? (
                  <FormHelperText error id="helper-tex-completion-ratio-label">
                    {errors.completion_ratio}
                  </FormHelperText>
                ) : (
                  <FormHelperText
                    id="helper-tex-completion-ratio-label"> {inputPrompt.completion_ratio} </FormHelperText>
                )}
              </FormControl>

              <DialogActions>
                <Button onClick={onCancel}>取消</Button>
                <Button disableElevation disabled={isSubmitting} type="submit" variant="contained" color="primary">
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

export default EditModal;

EditModal.propTypes = {
  open: PropTypes.bool,
  modelMetaId: PropTypes.number,
  onCancel: PropTypes.func,
  onOk: PropTypes.func
};
