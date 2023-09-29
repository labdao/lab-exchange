import { createSlice, PayloadAction } from '@reduxjs/toolkit';
// import { initWeb3Auth } from './thunks';

interface UserState {
  web3Auth: any;
  walletAddress: string;
  emailAddress: string;
  isLoading: boolean;
  error: string | null;
  isLoggedIn: boolean;
}

const initialState: UserState = {
  web3Auth: null,
  walletAddress: '',
  emailAddress: '',
  isLoading: false,
  error: null,
  isLoggedIn: false,
};

export const userSlice = createSlice({
  name: 'user',
  initialState,
  reducers: {
    setWalletAddress: (state, action: PayloadAction<string>) => {
      state.walletAddress = action.payload;
    },
    setEmailAddress: (state, action: PayloadAction<string>) => {
      state.emailAddress = action.payload;
    },
    setError: (state, action: PayloadAction<string | null>) => {
      state.error = action.payload;
    },
    startLoading: (state) => {
      state.isLoading = true;
    },
    endLoading: (state) => {
      state.isLoading = false;
    },
    setIsLoggedIn: (state, action: PayloadAction<boolean>) => {
      state.isLoggedIn = action.payload;
    },
  },
});

export const {
  setWalletAddress,
  setEmailAddress,
  setError,
  startLoading,
  endLoading,
  setIsLoggedIn,
} = userSlice.actions;

export default userSlice.reducer;