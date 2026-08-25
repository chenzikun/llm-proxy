// material-ui
import { Link, Container, Box } from '@mui/material';
import React from 'react';
import { useSelector } from 'react-redux';

// ==============================|| FOOTER - AUTHENTICATION 2 & 3 ||============================== //

const Footer = () => {
  const siteInfo = useSelector((state) => state.siteInfo);

  return (
    <Container sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '64px' }}>
      <Box sx={{ textAlign: 'center' }}>
        {siteInfo.footer_html ? (
          <div className="custom-footer" dangerouslySetInnerHTML={{ __html: siteInfo.footer_html }}></div>
        ) : (
          <>
            本系统基于开源框架 <Link href="https://github.com/songquanpeng/one-api" target="_blank">
              One API
            </Link>
             实现, 由 智能光储充-AI应用部 @陈子坤 @高健 维护

          </>
        )}
      </Box>
    </Container>
  );
};

export default Footer;
