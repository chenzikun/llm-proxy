import { useState, useEffect, useCallback } from 'react';
import { showError, showSuccess } from 'utils/common';

// import { useTheme } from '@mui/material/styles';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableContainer from '@mui/material/TableContainer';
import PerfectScrollbar from 'react-perfect-scrollbar';
import TablePagination from '@mui/material/TablePagination';
import LinearProgress from '@mui/material/LinearProgress';
// import useMediaQuery from '@mui/material/useMediaQuery';

import { Button, Card, Box, Stack, Typography } from '@mui/material';
import ModelMetaTableRow from './component/TableRow';
import ModelMetaTableHead from './component/TableHead';
import TableToolBar from 'ui-component/TableToolBar';
import { API } from 'utils/api';
import { ITEMS_PER_PAGE } from 'constants';
import { IconPlus, IconFileImport, IconFile } from '@tabler/icons-react';
import EditeModal from './component/EditModal';
import UploadFileModal from './component/UploadFileModal';
import BatchModal from './component/BatchModal';

// ----------------------------------------------------------------------
// CHANNEL_OPTIONS,
export default function ModelMetaPage() {
  const [modelMetas, setModelMetas] = useState([]);
  const [activePage, setActivePage] = useState(0);
  const [searching, setSearching] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  // const theme = useTheme();
  // const matchUpMd = useMediaQuery(theme.breakpoints.up('sm'));
  const [openModal, setOpenModal] = useState(false);
  const [openUploadFileModal, setOpenUploadFileModal] = useState(false);
  const [openBatchModal, setOpenBatchModal] = useState(false);
  const [editModelMetaId, setEditModeMetalId] = useState(0);

  const loadModelMetas = useCallback(async (startIdx) => {
    setSearching(true);
    const res = await API.get(`/api/model-meta/?p=${startIdx}`);
    const { success, message, data } = res.data;
    if (success) {
      if (startIdx === 0) {
        setModelMetas(data);
      } else {
        setModelMetas(prevMetas => {
          let newModelMetas = [...prevMetas];
          newModelMetas.splice(startIdx * ITEMS_PER_PAGE, data.length, ...data);
          return newModelMetas;
        });
      }
    } else {
      showError(message);
    }
    setSearching(false);
  }, [setSearching, setModelMetas]);

  const onPaginationChange = (event, activePage) => {
    (async () => {
      if (activePage === Math.ceil(modelMetas.length / ITEMS_PER_PAGE)) {
        // In this case we have to load more data and then append them.
        await loadModelMetas(activePage);
      }
      setActivePage(activePage);
    })();
  };

  const searchModelMeta = async (event) => {
    event.preventDefault();
    if (searchKeyword === '') {
      await loadModelMetas(0);
      setActivePage(0);
      return;
    }
    setSearching(true);
    const res = await API.get(`/api/model-meta/search?keyword=${searchKeyword}`);
    const { success, message, data } = res.data;
    if (success) {
      setModelMetas(data);
      setActivePage(0);
    } else {
      showError(message);
    }
    setSearching(false);
  };

  const handleSearchKeyword = (event) => {
    setSearchKeyword(event.target.value);
  };

  const manageModelMeta = async (id, action, value) => {
    const url = '/api/model-meta/';
    let data = { id };
    let res;
    switch (action) {
      case 'delete':
        res = await API.delete(url + id);
        break;
      case 'status':
        res = await API.put(url, {
          ...data,
          status: value
        });
        break;
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess('操作成功完成！');
      if (action === 'delete') {
        await handleRefresh();
      }
    } else {
      showError(message);
    }

    return res.data;
  };

  // 处理刷新
  const handleRefresh = async () => {
    await loadModelMetas(activePage);
  };

  // 提交模型
  const handleOkModal = (status) => {
    if (status === true) {
      handleCloseModal();
      handleRefresh();
    }
  };

  // 打开新增模型+编辑模型按钮
  const handleOpenModal = (ModelMetaId) => {
    setEditModeMetalId(ModelMetaId);
    setOpenModal(true);
  };

  const handleOpenBatchModal = () => {
    setOpenBatchModal(true);
  };

  const handleOpenUploadFileModal = () => {
    setOpenUploadFileModal(true);
  };

  // 取消提交模型
  const handleCloseModal = () => {
    setOpenModal(false);
    setEditModeMetalId(0);
  };

  // 取消上传文件
  const handleCloseUploadFileModal = () => {
    setOpenUploadFileModal(false);
  };

  // 处理上传文件成功
  const handleUploadFileSuccess = (status) => {
    if (status === true) {
      handleCloseUploadFileModal();
      handleRefresh();
    }
  };

  // 关闭批量添加模型模态框
  const handleCloseBatchModal = () => {
    setOpenBatchModal(false);
  };

  // 处理批量添加模型成功
  const handleBatchSuccess = (status) => {
    if (status === true) {
      handleCloseBatchModal();
      handleRefresh();
    }
  };

  useEffect(() => {
    loadModelMetas(0)
      .then()
      .catch((reason) => {
        showError(reason);
      });
    // loadChannelModels().then();
  }, [loadModelMetas]);

  return (
    <>
      <Stack direction="row" alignItems="center" justifyContent="space-between" mb={2.5}>
        <Typography variant="h4">模型</Typography>
        <div>
          <Button variant="contained" color="primary" startIcon={<IconFileImport />} onClick={() => handleOpenUploadFileModal(0)} sx={{ mr: 2 }}>
            上传文件
          </Button>
          <Button variant="contained" color="primary" startIcon={<IconFile />} onClick={() => handleOpenBatchModal(0)} sx={{ mr: 2 }}>
            批量添加
          </Button>
          <Button variant="contained" color="primary" startIcon={<IconPlus />} onClick={() => handleOpenModal(0)}>
            新增模型
          </Button>
        </div>
      </Stack>
      <Card>
        <Box component="form" onSubmit={searchModelMeta} noValidate sx={{ marginTop: 2 }}>
          <TableToolBar filterName={searchKeyword} handleFilterName={handleSearchKeyword} placeholder={'搜索模型的Name channel name ...'} />
        </Box>
        {searching && <LinearProgress />}
        <PerfectScrollbar component="div">
          <TableContainer sx={{ overflow: 'unset' }}>
            <Table sx={{ minWidth: 800 }}>
              <ModelMetaTableHead />
              <TableBody>
                {modelMetas.slice(activePage * ITEMS_PER_PAGE, (activePage + 1) * ITEMS_PER_PAGE).map((row) => (
                  <ModelMetaTableRow
                    item={row}
                    manageModelMeta={manageModelMeta}
                    key={row.id}
                    handleOpenModal={handleOpenModal}
                    setModeMetalId={setEditModeMetalId}
                  />
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </PerfectScrollbar>
        <TablePagination
          page={activePage}
          component="div"
          count={modelMetas.length + (modelMetas.length % ITEMS_PER_PAGE === 0 ? 1 : 0)}
          rowsPerPage={ITEMS_PER_PAGE}
          onPageChange={onPaginationChange}
          rowsPerPageOptions={[ITEMS_PER_PAGE]}
        />
      </Card>
      <EditeModal open={openModal} onCancel={handleCloseModal} onOk={handleOkModal} modelMetaId={editModelMetaId} />
      <UploadFileModal open={openUploadFileModal} onCancel={handleCloseUploadFileModal} onOk={handleUploadFileSuccess} />
      <BatchModal open={openBatchModal} onCancel={handleCloseBatchModal} onOk={handleBatchSuccess} />
    </>
  );
}
