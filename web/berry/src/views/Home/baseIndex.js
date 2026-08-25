import { Box, Typography, Container, Stack } from '@mui/material';
import Grid from '@mui/material/Unstable_Grid2';
// import { Note } from '@mui/icons-material';
// import { Link } from 'react-router-dom';

const BaseIndex = () => (
  <Box
    sx={{
      minHeight: 'calc(100vh - 136px)',
      backgroundImage: 'linear-gradient(to right, #ff9966, #ff5e62)',
      color: 'white',
      p: 4
    }}
  >
    <Container maxWidth="lg">
      <Grid container columns={12} wrap="nowrap" alignItems="center" sx={{ minHeight: 'calc(100vh - 230px)' }}>
        <Grid md={7} lg={6}>
          <Stack spacing={3}>
            <Typography variant="h1" sx={{ fontSize: '4rem', color: '#fff', lineHeight: 1.5 }}>
              One API
            </Typography>
            <Typography variant="h4" sx={{fontSize: '1.5rem', color: '#fff', lineHeight: 1.5}}>
              - LLM接入与管理平台 <br/>
              - 兼容多种供应商 <br/>
              - API Key 与 Token 管理 <br/>
            </Typography>
            {/* <Button
              variant="contained"
              component={Link}
              startIcon={<Note />}
              to="/doc"
              target="_blank"
              rel="noopener noreferrer"
              sx={{ backgroundColor: '#24292e', color: '#fff', width: 'fit-content', boxShadow: '0 3px 5px 2px rgba(255, 105, 135, .3)' }}
            >
              文档
            </Button> */}
          </Stack>
        </Grid>
      </Grid>
    </Container>
  </Box>
);

export default BaseIndex;
