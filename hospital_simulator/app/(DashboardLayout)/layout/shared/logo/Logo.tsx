import Link from "next/link";
import { styled } from "@mui/material";
import Image from "next/image";

const LinkStyled = styled(Link)(() => ({
  height: "49px",
  width: "213px",
  overflow: "hidden",
  display: "block",
  marginTop: "15px"
}));

const Logo = () => {
  return (
    <LinkStyled href="/">
      <Image src="/images/logos/logo.svg" alt="logo" height={49} width={213} priority />
    </LinkStyled>
  );
};

export default Logo;
