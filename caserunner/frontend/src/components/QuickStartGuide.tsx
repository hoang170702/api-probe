import { Alert, Button, Card, Flex, Image, Modal, Tag, Typography } from 'antd'
import { useI18n } from '../i18n/context'
import pasteCurl from '../../../../docs/images/01-paste-curl.png'
import requestAndFields from '../../../../docs/images/02-request-and-fields.png'
import testCases from '../../../../docs/images/06-test-cases.png'
import testCasesVI from '../../../../docs/images/06-test-cases-vi.png'
import results from '../../../../docs/images/07-results.png'
import resultsVI from '../../../../docs/images/07-results-vi.png'

type Props = {
  open: boolean
  onClose: () => void
}

/** A short, visual walkthrough for a tester opening API Case Runner for the first time. */
export default function QuickStartGuide({ open, onClose }: Props) {
  const { lang, t } = useI18n()
  const steps = [
    { number: 1, title: t('guide.paste.title'), body: t('guide.paste.body'), caption: t('guide.paste.caption'), image: pasteCurl },
    { number: 2, title: t('guide.review.title'), body: t('guide.review.body'), caption: t('guide.review.caption'), image: requestAndFields },
    { number: 3, title: t('guide.cases.title'), body: t('guide.cases.body'), caption: t('guide.cases.caption'), image: lang === 'vi' ? testCasesVI : testCases },
    { number: 4, title: t('guide.results.title'), body: t('guide.results.body'), caption: t('guide.results.caption'), image: lang === 'vi' ? resultsVI : results },
  ]

  return (
    <Modal
      open={open}
      onCancel={onClose}
      destroyOnHidden
      width="min(1180px, calc(100vw - 32px))"
      style={{ top: 32 }}
      title={t('guide.title')}
      footer={<Button onClick={onClose}>{t('guide.close')}</Button>}
    >
      <Flex vertical gap={20}>
        <Typography.Paragraph type="secondary" style={{ margin: 0 }}>
          {t('guide.intro')}
        </Typography.Paragraph>
        <Flex gap={16} wrap align="stretch">
          {steps.map((step) => (
            <Card
              key={step.number}
              size="small"
              style={{ flex: '1 1 240px', minWidth: 0 }}
              styles={{ body: { display: 'flex', flexDirection: 'column', gap: 12 } }}
            >
              <Flex align="center" gap={8}>
                <Tag color="blue" style={{ margin: 0, borderRadius: 12 }}>
                  {step.number}
                </Tag>
                <Typography.Text strong>{step.title}</Typography.Text>
              </Flex>
              <Typography.Text type="secondary">{step.body}</Typography.Text>
              <Image
                alt={step.caption}
                src={step.image}
                preview={{ mask: t('guide.preview') }}
                style={{ width: '100%', height: 150, objectFit: 'cover', objectPosition: 'top', border: '1px solid #e5e7eb', borderRadius: 6 }}
              />
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {step.caption}
              </Typography.Text>
            </Card>
          ))}
        </Flex>
        <Alert showIcon type="info" title={t('guide.token.title')} description={t('guide.token.body')} />
      </Flex>
    </Modal>
  )
}
