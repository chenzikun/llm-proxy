import PropTypes from 'prop-types';
import { useState } from 'react';

import { timestamp2string } from 'utils/common';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';

import {
  Popover,
  TableRow,
  MenuItem,
  TableCell,
  IconButton,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Tooltip,
  Button
} from '@mui/material';

import Label from 'ui-component/Label';
import TableSwitch from 'ui-component/Switch';


import { IconDotsVertical, IconEdit, IconTrash } from '@tabler/icons-react';

export default function ModelMetaTableRow({
  item,
  manageModelMeta,
  handleOpenModal,
  setModeMetalId
}) {
  const [open, setOpen] = useState(null);
  const [openDelete, setOpenDelete] = useState(false);
  const [statusSwitch, setStatusSwitch] = useState(item.status);

  const handleDeleteOpen = () => {
    handleCloseMenu();
    setOpenDelete(true);
  };

  const handleDeleteClose = () => {
    setOpenDelete(false);
  };

  const handleOpenMenu = (event) => {
    setOpen(event.currentTarget);
  };

  const handleCloseMenu = () => {
    setOpen(null);
  };

  const handleStatus = async () => {
    const switchVlue = statusSwitch === 1 ? 2 : 1;
    const { success } = await manageModelMeta(item.id, 'status', switchVlue);
    if (success) {
      setStatusSwitch(switchVlue);
    }
  };

  const handleDelete = async () => {
    handleCloseMenu();
    await manageModelMeta(item.id, 'delete', '');
  };

  return (
    <>
      <TableRow tabIndex={item.id}>
        <TableCell>{item.id}</TableCell>
        <TableCell>{item.model}</TableCell>

        {/*channel-渠道*/}
        <TableCell>
          {!CHANNEL_OPTIONS[item.channel_type] ? (
            <Label color="error" variant="outlined">
              未知
            </Label>
          ) : (
            <Label color={CHANNEL_OPTIONS[item.channel_type].color} variant="outlined">
              {CHANNEL_OPTIONS[item.channel_type].text}
            </Label>
          )}
        </TableCell>

        {/*状态*/}
        <TableCell>
          <Tooltip
            title={(() => {
              switch (statusSwitch) {
                case 1:
                  return '已启用';
                case 2:
                  return '本模型被手动禁用';
                case 3:
                  return '本模型被程序自动禁用';
                default:
                  return '未知';
              }
            })()}
            placement="top"
          >
            <TableSwitch
              id={`switch-${item.id}`}
              checked={statusSwitch === 1}
              onChange={handleStatus}
            />
          </Tooltip>
        </TableCell>

        <TableCell>{timestamp2string(item.created_time)}</TableCell>
        <TableCell>{renderPrice(item.input_price, item.price_unit)}</TableCell>
        <TableCell>{renderPrice(item.output_price, item.price_unit)}</TableCell>
        <TableCell>{renderPrice(item.cache_price, item.price_unit)}</TableCell>
        <TableCell>{item.price_unit === 'USD' ? '$ USD' : '¥ CNY'}</TableCell>


        <TableCell>
          <IconButton
            onClick={handleOpenMenu}
            sx={{ color: 'rgb(99, 115, 129)' }}
          >
            <IconDotsVertical />
          </IconButton>
        </TableCell>
      </TableRow>

      <Popover
        open={!!open}
        anchorEl={open}
        onClose={handleCloseMenu}
        anchorOrigin={{ vertical: 'top', horizontal: 'left' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        PaperProps={{
          sx: { width: 140 }
        }}
      >
        <MenuItem
          onClick={() => {
            handleCloseMenu();
            handleOpenModal();
            setModeMetalId(item.id);
          }}
        >
          <IconEdit style={{ marginRight: '16px' }} />
          编辑
        </MenuItem>
        <MenuItem onClick={handleDeleteOpen} sx={{ color: 'error.main' }}>
          <IconTrash style={{ marginRight: '16px' }} />
          删除
        </MenuItem>
      </Popover>

      <Dialog open={openDelete} onClose={handleDeleteClose}>
        <DialogTitle>删除模型</DialogTitle>
        <DialogContent>
          <DialogContentText>是否删除模型 {item.model}？</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleDeleteClose}>关闭</Button>
          <Button onClick={handleDelete} sx={{ color: 'error.main' }} autoFocus>
            删除
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}

ModelMetaTableRow.propTypes = {
  item: PropTypes.object,
  manageModelMeta: PropTypes.func,
  handleOpenModal: PropTypes.func,
  setModeMetalId: PropTypes.func
};

function renderPrice(price, priceUnit) {
  const symbol = priceUnit === 'USD' ? '$' : '¥';
  const val = typeof price === 'number' ? price : 0;
  return <span>{symbol}{val.toFixed(4)}/1K</span>;
}
