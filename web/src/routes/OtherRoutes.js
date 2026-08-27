import { lazy } from 'react';

// project imports
import Loadable from 'ui-component/Loadable';
import MinimalLayout from 'layout/MinimalLayout';

// login option 3 routing
const AuthLogin = Loadable(lazy(() => import('views/Authentication/Auth/Login')));
const AuthLoginAdmin = Loadable(lazy(() => import('views/Authentication/Auth/LoginAdmin')));
const AuthRegister = Loadable(lazy(() => import('views/Authentication/Auth/Register')));
const GitHubOAuth = Loadable(lazy(() => import('views/Authentication/Auth/GitHubOAuth')));
const LarkOAuth = Loadable(lazy(() => import('views/Authentication/Auth/LarkOAuth')));
const OidcOAuth = Loadable(lazy(() => import('views/Authentication/Auth/OidcOAuth')));
const ForgetPassword = Loadable(lazy(() => import('views/Authentication/Auth/ForgetPassword')));
const ResetPassword = Loadable(lazy(() => import('views/Authentication/Auth/ResetPassword')));
const Home = Loadable(lazy(() => import('views/Home')));
const About = Loadable(lazy(() => import('views/About')));
const Document = Loadable(lazy(() => import('views/Document')));
const NotFoundView = Loadable(lazy(() => import('views/Error')));

// ==============================|| AUTHENTICATION ROUTING ||============================== //

const OtherRoutes = {
  path: '/',
  element: <MinimalLayout />,
  children: [
    {
      path: '',
      element: <Home />
    },
    {
      path: '/about',
      element: <About />
    },
    {
      path: 'document',
      element: <Document />
    },
    {
      path: '/login',
      element: <AuthLogin />
    },
    {
      path: '/admin',
      element: <AuthLoginAdmin />
    },
    {
      path: '/register',
      element: <AuthRegister />
    },
    {
      path: '/reset',
      element: <ForgetPassword />
    },
    {
      path: '/user/reset',
      element: <ResetPassword />
    },
    {
      path: '/oauth/github',
      element: <GitHubOAuth />
    },
    {
      path: '/oauth/lark',
      element: <LarkOAuth />
    },
    {
      path: 'oauth/oidc',
      element: <OidcOAuth />
    },
    {
      path: '/404',
      element: <NotFoundView />
    }
  ]
};

export default OtherRoutes;
