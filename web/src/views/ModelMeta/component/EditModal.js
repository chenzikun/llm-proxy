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
  Stack,
} from '@mui/material';

import { Formik } from 'formik';
import * as Yup from 'yup';
import { defaultConfig, BILLING_UNITS } from '../type/Config'; //typeConfig


const floatSchema = Yup.number().min(0, '价格不能为负数').transform((value, originalValue) => {
  if (typeof originalValue === 'string' && originalValue.includes('.')) {
    return parseFloat(originalValue);
  }
  return value;
});

const validationSchema = Yup.object().shape({
  channel_type: Yup.number().required('渠道 不能为空'),
  model: Yup.string().required('名称 不能为空'),
  status: Yup.number(),
  input_price: floatSchema,
  output_price: floatSchema,
  cache_price: floatSchema,
  price_unit: Yup.string().oneOf(['CNY', 'USD'], '请选择有效的货币单位'),
  billing_unit: Yup.string().oneOf(['token', 'char', 'second', 'image'], '请选择有效的计量单位'),
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

  // billing_unit=image 时后端存的是“每百万张”的价格，界面按“每张”展示，
  // 避免管理员为 ¥0.3/张 填写 300000 这种反直觉的数字
  const toDisplayPrice = (value, unit) => (unit === 'image' ? (Number(value) || 0) / 1000000 : Number(value) || 0);
  const toStoredPrice = (value, unit) => (unit === 'image' ? (Number(value) || 0) * 1000000 : Number(value) || 0);

  const priceSuffix = (unit) => {
    const found = BILLING_UNITS.find((u) => u.value === unit);
    return found ? found.priceUnit : '百万 token';
  };

  const submit = async (values, { setErrors, setStatus, setSubmitting }) => {
    setSubmitting(true);

    let res;
    const unit = values.billing_unit || 'token';
    values.input_price = toStoredPrice(values.input_price, unit);
    values.output_price = toStoredPrice(values.output_price, unit);
    values.cache_price = toStoredPrice(values.cache_price, unit);
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
        const unit = data.billing_unit || 'token';
        setInitialInput({
          ...data,
          billing_unit: unit,
          input_price: toDisplayPrice(data.input_price, unit),
          output_price: toDisplayPrice(data.output_price, unit),
          cache_price: toDisplayPrice(data.cache_price, unit)
        });
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

              {/*计量单位*/}
              <FormControl fullWidth error={Boolean(touched.billing_unit && errors.billing_unit)}
                           sx={{ ...theme.typography.otherInput }}>
                <InputLabel htmlFor="billing-unit-label">{inputLabel.billing_unit}</InputLabel>
                <Select
                  id="billing-unit-label"
                  label={inputLabel.billing_unit}
                  value={values.billing_unit || 'token'}
                  name="billing_unit"
                  onBlur={handleBlur}
                  onChange={handleChange}
                >
                  {BILLING_UNITS.map((u) => (
                    <MenuItem key={u.value} value={u.value}>
                      {u.label}
                    </MenuItem>
                  ))}
                </Select>
                <FormHelperText id="helper-tex-billing-unit-label">{inputPrompt.billing_unit}</FormHelperText>
              </FormControl>

              {/*价格单位*/}
              <FormControl fullWidth error={Boolean(touched.price_unit && errors.price_unit)}
                           sx={{ ...theme.typography.otherInput }}>
                <InputLabel htmlFor="price-unit-label">{inputLabel.price_unit}</InputLabel>
                <Select
                  id="price-unit-label"
                  label={inputLabel.price_unit}
                  value={values.price_unit}
                  name="price_unit"
                  onBlur={handleBlur}
                  onChange={handleChange}
                >
                  <MenuItem value="CNY">¥ 人民币（CNY）</MenuItem>
                  <MenuItem value="USD">$ 美元（USD）</MenuItem>
                </Select>
                <FormHelperText id="helper-tex-price-unit-label">{inputPrompt.price_unit}</FormHelperText>
              </FormControl>

              {/*价格字段 — 一行三列*/}
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ ...theme.typography.otherInput }}>
                {/*输入价格*/}
                <FormControl fullWidth error={Boolean(touched.input_price && errors.input_price)}>
                  <InputLabel htmlFor="input-price-label">
                    {`${inputLabel.input_price}（每${priceSuffix(values.billing_unit)}）`}
                  </InputLabel>
                  <OutlinedInput
                    id="input-price-label"
                    label={`${inputLabel.input_price}（每${priceSuffix(values.billing_unit)}）`}
                    type="number"
                    value={values.input_price}
                    name="input_price"
                    onBlur={handleBlur}
                    onChange={handleChange}
                    inputProps={{ step: 'any', min: 0 }}
                  />
                  {touched.input_price && errors.input_price ? (
                    <FormHelperText error>{errors.input_price}</FormHelperText>
                  ) : (
                    <FormHelperText>
                      {values.billing_unit === 'image'
                        ? '按张计价时该字段不参与计算'
                        : inputPrompt.input_price}
                    </FormHelperText>
                  )}
                </FormControl>

                {/*输出价格*/}
                <FormControl fullWidth error={Boolean(touched.output_price && errors.output_price)}>
                  <InputLabel htmlFor="output-price-label">
                    {`${inputLabel.output_price}（每${priceSuffix(values.billing_unit)}）`}
                  </InputLabel>
                  <OutlinedInput
                    id="output-price-label"
                    label={`${inputLabel.output_price}（每${priceSuffix(values.billing_unit)}）`}
                    type="number"
                    value={values.output_price}
                    name="output_price"
                    onBlur={handleBlur}
                    onChange={handleChange}
                    inputProps={{ step: 'any', min: 0 }}
                  />
                  {touched.output_price && errors.output_price ? (
                    <FormHelperText error>{errors.output_price}</FormHelperText>
                  ) : (
                    <FormHelperText>{inputPrompt.output_price}</FormHelperText>
                  )}
                </FormControl>

                {/*缓存价格*/}
                <FormControl fullWidth error={Boolean(touched.cache_price && errors.cache_price)}>
                  <InputLabel htmlFor="cache-price-label">
                    {`${inputLabel.cache_price}（每${priceSuffix(values.billing_unit)}）`}
                  </InputLabel>
                  <OutlinedInput
                    id="cache-price-label"
                    label={`${inputLabel.cache_price}（每${priceSuffix(values.billing_unit)}）`}
                    type="number"
                    value={values.cache_price}
                    name="cache_price"
                    onBlur={handleBlur}
                    onChange={handleChange}
                    inputProps={{ step: 'any', min: 0 }}
                  />
                  {touched.cache_price && errors.cache_price ? (
                    <FormHelperText error>{errors.cache_price}</FormHelperText>
                  ) : (
                    <FormHelperText>
                      {values.billing_unit === 'image'
                        ? '按张计价时该字段不参与计算'
                        : values.cache_price > 0
                        ? `✓ 已启用缓存折扣：命中缓存的 token 按此价格计费，其余按输入价格`
                        : `⚠ 未配置（= 0）：缓存命中的 token 仍按输入价格收费，不享受折扣`}
                    </FormHelperText>
                  )}
                </FormControl>
              </Stack>

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
