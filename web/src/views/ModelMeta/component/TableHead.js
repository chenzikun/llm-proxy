import { TableCell, TableHead, TableRow } from '@mui/material';

const ModelMetaTableHead = () => {
  return (
    <TableHead>
      <TableRow>
        <TableCell>ID</TableCell>
        <TableCell>名称</TableCell>
        <TableCell>渠道</TableCell>
        <TableCell>状态</TableCell>
        <TableCell>创建时间</TableCell>
        <TableCell>输入价格</TableCell>
        <TableCell>输出价格</TableCell>
        <TableCell>缓存价格</TableCell>
        <TableCell>计量单位</TableCell>
        <TableCell>单位</TableCell>
        <TableCell>操作</TableCell>
      </TableRow>
    </TableHead>
  );
};

export default ModelMetaTableHead;
