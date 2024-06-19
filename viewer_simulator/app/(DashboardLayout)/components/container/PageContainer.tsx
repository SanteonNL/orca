type Props = {
  description?: string;
  children: JSX.Element | JSX.Element[];
  title?: string;
};

const PageContainer = ({ title, description, children }: Props) => (
  <div>
    <title>{title}</title>
    <meta name="description" content={description} />
    {children}
  </div>
);

export default PageContainer;
