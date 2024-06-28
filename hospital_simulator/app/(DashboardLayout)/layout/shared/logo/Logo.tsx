import Link from "next/link";
import { styled } from "@mui/material";
import Image from "next/image";

const LinkStyled = styled(Link)(() => ({
  height: "97px",
  width: "213px",
  overflow: "hidden",
  display: "block",
  marginTop: "15px"
}));

const Logo = () => {
  return (
    <LinkStyled href="/">
      <Image src={`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/images/logos/logo.png`} alt="logo" height={97} width={213} priority />
    </LinkStyled>
  );
};

export default Logo;
